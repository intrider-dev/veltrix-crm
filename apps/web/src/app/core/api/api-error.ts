import type { HttpErrorResponse } from '@angular/common/http';

import type { Problem } from './api.types';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly problem: Problem | null,
  ) {
    super(problem?.code ?? 'network');
  }

  static from(response: HttpErrorResponse): ApiError {
    return new ApiError(response.status, isProblem(response.error) ? response.error : null);
  }
}

function isProblem(value: unknown): value is Problem {
  if (typeof value !== 'object' || value === null) return false;
  return 'code' in value && typeof value.code === 'string' && 'status' in value;
}
