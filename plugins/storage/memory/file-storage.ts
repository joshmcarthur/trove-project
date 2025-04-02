import { EventFile, FileStorage } from "@trove/core/types.ts";

export class MemoryFileStorage implements FileStorage {
  private files: Map<string, EventFile> = new Map();

  async initialize(): Promise<void> {
    // No initialization needed for memory storage
  }

  saveFile(file: EventFile): Promise<string> {
    const fileId = file.id || crypto.randomUUID();
    this.files.set(fileId, { ...file, id: fileId });
    return Promise.resolve(fileId);
  }

  getFile(fileId: string): Promise<EventFile | null> {
    return Promise.resolve(this.files.get(fileId) || null);
  }

  getFileData(fileId: string): Promise<Uint8Array | string> {
    const file = this.files.get(fileId);
    if (!file) throw new Error(`File ${fileId} not found`);
    return Promise.resolve(file.data);
  }
}
