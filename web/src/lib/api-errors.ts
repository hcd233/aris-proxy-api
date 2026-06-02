/**
 * 前后端共享的业务错误码常量。
 *
 * 后端定义在 `internal/common/ierr/sentinels.go`，前端在此处镜像。
 * 新增/修改业务错误码时必须同步更新两侧。
 */
export const BusinessErrorCode = {
  /** 未授权（token 无效、缺失或过期） */
  Unauthorized: 10001,
} as const;

export type BusinessErrorCode = (typeof BusinessErrorCode)[keyof typeof BusinessErrorCode];
