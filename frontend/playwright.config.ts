import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  timeout: 60000,
  use: { baseURL: "http://localhost:5173", browserName: "chromium" },
  webServer: [
    {
      command: "go run .",
      cwd: "../backend",
      url: "http://127.0.0.1:8080/ready",
      reuseExistingServer: true,
      env: { CHAT_HANDSHAKE_BURST: "1000", CHAT_HANDSHAKES_PER_MINUTE: "1000" },
    },
    { command: "npm run dev", url: "http://localhost:5173", reuseExistingServer: true },
  ],
});
