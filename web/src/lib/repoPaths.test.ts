import { describe, expect, it } from 'vitest';
import { formatSize, splitRefPath } from './repoPaths';

describe('splitRefPath', () => {
  const branches = ['main', 'feature', 'feature/login'];
  it('uses the default branch for an empty tail', () => {
    expect(splitRefPath('', branches, 'main')).toEqual(['main', '']);
  });
  it('picks the longest branch prefix', () => {
    expect(splitRefPath('feature/login/src/app.ts', branches, 'main')).toEqual(['feature/login', 'src/app.ts']);
    expect(splitRefPath('feature/other.txt', branches, 'main')).toEqual(['feature', 'other.txt']);
  });
  it('falls back to the first segment (a sha)', () => {
    expect(splitRefPath('abc123/docs', branches, 'main')).toEqual(['abc123', 'docs']);
  });
});

describe('formatSize', () => {
  it('uses binary units', () => {
    expect(formatSize(512)).toBe('512 B');
    expect(formatSize(2048)).toBe('2.0 KiB');
    expect(formatSize(3 * 1024 * 1024)).toBe('3.0 MiB');
  });
});
