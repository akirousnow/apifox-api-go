/** Health */
// GET /health

// ====== 返回响应 ======
// 200 application/json
export type HealthCheckResponse = {
  status: "up" | "down";
  meta?: {
    /** build version */
    version?: string;
  };
  tags?: Array<string>;
  nullableName?: string | null;
};
