export type ParsedDiffFile = {
  path: string
  lines: string[]
  additions: number
  deletions: number
}

export function parseDiffFiles(patch: string): ParsedDiffFile[] {
  if (!patch.trim()) {
    return []
  }

  const lines = patch.split('\n')
  const files: ParsedDiffFile[] = []
  let current: ParsedDiffFile | null = null

  for (const line of lines) {
    if (line.startsWith('diff --git ')) {
      if (current) {
        files.push(current)
      }
      current = {
        path: extractDiffPath(line),
        lines: [line],
        additions: 0,
        deletions: 0,
      }
      continue
    }

    if (!current) {
      current = { path: 'commit', lines: [], additions: 0, deletions: 0 }
    }

    if (!line.startsWith('+++') && line.startsWith('+')) {
      current.additions += 1
    } else if (!line.startsWith('---') && line.startsWith('-')) {
      current.deletions += 1
    }
    current.lines.push(line)
  }

  if (current) {
    files.push(current)
  }

  return files
}

function extractDiffPath(line: string) {
  const parts = line.trim().split(' ')
  const bPath = parts[3] ?? ''
  if (bPath.startsWith('b/')) {
    return bPath.slice(2)
  }
  return bPath || 'unknown'
}

export function diffLineTone(line: string) {
  if (line.startsWith('+++') || line.startsWith('---') || line.startsWith('diff --git') || line.startsWith('@@')) {
    return 'font-mono text-sky-300'
  }
  if (line.startsWith('+')) {
    return 'font-mono text-emerald-300'
  }
  if (line.startsWith('-')) {
    return 'font-mono text-rose-300'
  }
  return 'font-mono text-stone-200'
}
