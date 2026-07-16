// ═════════ GET /users/{id} ═════════

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

// ═════════ POST /users/{id} ═════════

/** Create user */
// POST /users/{id}

// ====== 请求参数 ======

// Path 参数（URL 路径占位）
export interface CreateUserPathParams {
  /** user id */
  id: string;
}

// ====== 请求体 ======
// application/json
export type CreateUserRequestBody = UserInput;

// ====== 返回响应 ======
// 201 application/json
export type CreateUserResponse = User;

// ═════════ DELETE /users/{id} ═════════

/** Delete user */
// DELETE /users/{id}

// ====== 请求参数 ======

// Path 参数（URL 路径占位）
export interface DeleteUserPathParams {
  /** user id */
  id: string;
}

// 未找到 2xx JSON 响应，未生成 Response 类型。

// ====== 关联类型定义 ======
export interface User {
  /** primary key */
  id: string;
  name: string;
  profile?: Profile;
}
export interface UserInput {
  name: string;
  profile?: Profile;
}
export interface Profile {
  bio?: string;
  age?: number | null;
}
