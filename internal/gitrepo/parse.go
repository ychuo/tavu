package gitrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func changeID(kind Kind, path, oldPath string) string {
	sum := sha256.Sum256([]byte(string(kind) + "\x00" + path + "\x00" + oldPath))
	return hex.EncodeToString(sum[:])
}

// ParseStatus parses `git status --porcelain=v1 -z`. Under -z rename paths
// appear destination first, then source; names are raw bytes, never C-unquoted.
func ParseStatus(data []byte) ([]Change, error) {
	var changes []Change
	for len(data) > 0 {
		i := bytes.IndexByte(data, 0)
		if i < 0 {
			return nil, fmt.Errorf("Git status is missing a NUL terminator")
		}
		record := data[:i]
		data = data[i+1:]
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("Invalid Git status record")
		}
		x, y, path := record[0], record[1], string(record[3:])
		oldPath := ""
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			i = bytes.IndexByte(data, 0)
			if i < 0 {
				return nil, fmt.Errorf("Git rename record is missing its source path")
			}
			oldPath, data = string(data[:i]), data[i+1:]
			if oldPath == "" {
				return nil, fmt.Errorf("Git rename source path is empty")
			}
		}
		add := func(kind Kind, code, old string) {
			changes = append(changes, Change{ID: changeID(kind, path, old), Kind: kind, Path: path, OldPath: old, Code: code})
		}
		if x == '?' && y == '?' {
			path = strings.TrimSuffix(path, "/")
			add(Untracked, "?", "")
			continue
		}
		if x == '!' && y == '!' {
			continue
		}
		if x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
			add(Conflict, string(record[:2]), oldPath)
			continue
		}
		if x != ' ' {
			old := ""
			if x == 'R' || x == 'C' {
				old = oldPath
			}
			add(Staged, string(x), old)
		}
		if y != ' ' {
			old := ""
			if y == 'R' || y == 'C' {
				old = oldPath
			}
			add(Unstaged, string(y), old)
		}
	}
	return changes, nil
}
