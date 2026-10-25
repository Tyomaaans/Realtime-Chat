package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type event struct {
	Type    string `json:"type"`
	RoomID  string `json:"room_id"`
	Payload struct {
		Text   string `json:"text"`
		Sender string `json:"sender"`
	} `json:"payload"`
}

type visitor struct {
	conn    *websocket.Conn
	room    string
	matched chan string
}

type results struct {
	Phase        string  `json:"phase"`
	PolicyCloses int64   `json:"policy_closes"`
	Sockets      int     `json:"sockets"`
	Duration     string  `json:"duration"`
	Burst        bool    `json:"burst"`
	Sent         int64   `json:"sent"`
	Received     int64   `json:"received"`
	Errors       int64   `json:"errors"`
	P50MS        float64 `json:"p50_ms"`
	P95MS        float64 `json:"p95_ms"`
	CPUPercent   float64 `json:"mean_cpu_percent"`
	PeakRSSKiB   int64   `json:"peak_rss_kib"`
	RSSAfterKiB  int64   `json:"rss_after_kib"`
	Samples      int     `json:"latency_samples"`
}

func main() {
	address := flag.String("url", "ws://127.0.0.1:8081/v1/chat/ws", "local WebSocket URL")
	count := flag.Int("sockets", 100, "even number of visitors")
	duration := flag.Duration("duration", 30*time.Second, "sustained traffic duration")
	interval := flag.Duration("interval", 5*time.Second, "message interval per visitor")
	joinInterval := flag.Duration("join-interval", 10*time.Millisecond, "spacing between queue joins; zero stresses command admission")
	serverPID := flag.Int("server-pid", 0, "Linux backend process ID for CPU and RSS sampling")
	burst := flag.Bool("burst", false, "send a five-message burst per visitor before sustained traffic")
	flag.Parse()
	if *count < 2 || *count%2 != 0 || *duration < time.Second || *interval < 500*time.Millisecond {
		fmt.Fprintln(os.Stderr, "use an even socket count, positive duration, and an interval of at least 500ms")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var readers, writers sync.WaitGroup
	var sent, received, failures, policyCloses atomic.Int64
	var latencyMu sync.Mutex
	latencies := make([]float64, 0)
	visitors := make([]*visitor, 0, *count)
	for range *count {
		conn, response, err := websocket.DefaultDialer.Dial(*address, http.Header{"Origin": []string{"http://localhost:5173"}})
		if response != nil && response.Body != nil {
			response.Body.Close()
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, "connection failed; verify the explicit load-test limits:", err)
			os.Exit(1)
		}

		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var ready event
		if err := conn.ReadJSON(&ready); err != nil || ready.Type != "session.ready" {
			fmt.Fprintln(os.Stderr, "session did not become ready")
			os.Exit(1)
		}

		conn.SetReadDeadline(time.Time{})
		v := &visitor{
			conn:    conn,
			matched: make(chan string, 1),
		}
		visitors = append(visitors, v)
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				var incoming event
				if err := conn.ReadJSON(&incoming); err != nil {
					if ctx.Err() == nil {
						failures.Add(1)
						if websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
							policyCloses.Add(1)
						}
					}

					return
				}

				switch incoming.Type {
				case "chat.matched":
					v.matched <- incoming.RoomID
				case "chat.message":
					if incoming.Payload.Sender == "peer" {
						started, err := strconv.ParseInt(incoming.Payload.Text, 10, 64)
						if err != nil {
							failures.Add(1)
							continue
						}

						latency := float64(time.Since(time.Unix(0, started)).Microseconds()) / 1000
						latencyMu.Lock()
						latencies = append(latencies, latency)
						latencyMu.Unlock()
						received.Add(1)
					}
				case "error", "peer.left":
					if ctx.Err() == nil {
						failures.Add(1)
					}
				}
			}
		}()
	}

	for _, v := range visitors {
		if err := v.conn.WriteJSON(map[string]any{
			"type":       "queue.join",
			"request_id": "join",
			"payload":    struct{}{},
		}); err != nil {
			failures.Add(1)
		}

		time.Sleep(*joinInterval)
	}

	matchDeadline := time.NewTimer(10 * time.Second)
	for _, v := range visitors {
		select {
		case v.room = <-v.matched:
		case <-matchDeadline.C:
			json.NewEncoder(os.Stdout).Encode(results{
				Phase:        "matching",
				Sockets:      *count,
				Errors:       failures.Load(),
				PolicyCloses: policyCloses.Load(),
			})
			fmt.Fprintln(os.Stderr, "matching timed out")
			os.Exit(1)
		}
	}

	matchDeadline.Stop()
	startCPU, baselineRSS := processStats(*serverPID)
	peakRSS := baselineRSS
	stopMonitor := make(chan struct{})
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopMonitor:
				return
			case <-ticker.C:
				_, rss := processStats(*serverPID)
				peakRSS = max(peakRSS, rss)
			}
		}
	}()
	stopSend := make(chan struct{})
	for i, v := range visitors {
		writers.Add(1)
		go func() {
			defer writers.Done()
			send := func() {
				v.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				err := v.conn.WriteJSON(map[string]any{
					"type": "chat.send", "request_id": "send", "room_id": v.room,
					"payload": map[string]string{"text": strconv.FormatInt(time.Now().UnixNano(), 10)},
				})
				if err != nil {
					failures.Add(1)
					return
				}

				sent.Add(1)
			}
			if *burst {
				for range 5 {
					send()
				}
			}

			timer := time.NewTimer(*interval + time.Duration(i)*(*interval)/time.Duration(*count))
			defer timer.Stop()
			for {
				select {
				case <-stopSend:
					return
				case <-timer.C:
					send()
					timer.Reset(*interval)
				}
			}
		}()
	}

	<-time.NewTimer(*duration).C
	close(stopSend)
	writers.Wait()
	endCPU, _ := processStats(*serverPID)
	close(stopMonitor)
	<-monitorDone
	deadline := time.Now().Add(2 * time.Second)
	for received.Load() < sent.Load() && time.Now().Before(deadline) {
		<-time.NewTimer(10 * time.Millisecond).C
	}

	cancel()
	for _, v := range visitors {
		v.conn.Close()
	}

	readers.Wait()
	<-time.NewTimer(6 * time.Second).C
	_, rssAfter := processStats(*serverPID)
	latencyMu.Lock()
	sort.Float64s(latencies)
	percentile := func(fraction float64) float64 {
		if len(latencies) == 0 {
			return 0
		}

		return latencies[int(float64(len(latencies)-1)*fraction)]
	}
	result := results{
		Phase: "relay", PolicyCloses: policyCloses.Load(),
		Sockets: *count, Duration: duration.String(), Burst: *burst,
		Sent: sent.Load(), Received: received.Load(), Errors: failures.Load(),
		P50MS: percentile(0.50), P95MS: percentile(0.95), Samples: len(latencies),
		CPUPercent: (endCPU - startCPU) / duration.Seconds() * 100,
		PeakRSSKiB: peakRSS, RSSAfterKiB: rssAfter,
	}
	latencyMu.Unlock()
	json.NewEncoder(os.Stdout).Encode(result)
	if result.Errors > 0 || result.Sent != result.Received || result.Samples == 0 {
		os.Exit(1)
	}
}

func processStats(pid int) (float64, int64) {
	if pid <= 0 {
		return 0, 0
	}

	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0
	}

	end := strings.LastIndexByte(string(stat), ')')
	fields := strings.Fields(string(stat)[end+1:])
	if len(fields) < 13 {
		return 0, 0
	}

	user, _ := strconv.ParseFloat(fields[11], 64)
	system, _ := strconv.ParseFloat(fields[12], 64)
	status, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	var rss int64
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			parts := strings.Fields(line)
			rss, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}

	return (user + system) / 100, rss
}
