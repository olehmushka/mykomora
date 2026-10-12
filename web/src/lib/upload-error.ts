/** A direct-to-storage upload that the storage endpoint refused. */
export class UploadError extends Error {
  constructor(readonly status: number) {
    super(`upload failed (${status})`);
  }
}
