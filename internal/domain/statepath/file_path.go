package statepath

import "path/filepath"

// Path는 상태 키를 파일 경로로 조합한다. 순수 계산이므로 domain 계층이 소유한다.
func Path(dir, key string) string {
	return filepath.Join(dir, key+".json")
}
