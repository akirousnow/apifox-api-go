/** Get user */
// GET /users/{id}

// ====== 请求参数 ======

// Query 参数（URL 问号后）
export interface GetUserQuery {
  /** include details */
  verbose?: boolean;
}

// Path 参数（URL 路径占位）
export interface GetUserPathParams {
  /** user id */
  id: string;
}

// Header 参数（请求头）
export interface GetUserHeaders {
  "X-Trace"?: string;
}

// Cookie 参数
export interface GetUserCookies {
  session?: string;
}

// ====== 返回响应 ======
// 200 application/json
export type GetUserResponse = User;

// ====== 关联类型定义 ======
export interface User {
  /** primary key */
  id: string;
  name: string;
  profile?: Profile;
}
export interface Profile {
  bio?: string;
  age?: number | null;
}
