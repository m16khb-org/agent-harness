package architecture

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// orphanPackageAllowlist는 프로덕션 Go 파일을 갖고도 import되지 않는 것이
// 의도된 패키지다. 테스트 지원 전용이며 런타임 계약을 소유하지 않는다.
// 항목을 추가할 때는 왜 런타임에서 도달할 필요가 없는지 함께 적는다.
var orphanPackageAllowlist = map[string]string{
	"internal/adapter/skillcontract": "skill 텍스트 계약 테스트만 담는 doc-only 패키지",
	"cmd/harness/apidoc/dogfood":     "api-doc 테스트가 쓰는 dogfood fixture",
}

// TestProductionPackagesHaveImporters는 프로덕션 코드를 가진 패키지가 모듈
// 안에서 아무에게도 import되지 않는 상태를 막는다.
//
// 배선 파일이 지워지면서 구현만 남으면 그 패키지는 유령이 된다. 패키지의
// 테스트는 계속 초록이라 CI는 가드가 살아 있다고 보고하지만 런타임에서는
// 아무것도 강제되지 않는다. 2026-08-27 legacy hook 제거(2e810e13)가
// internal/adapter/remoteartifact를 정확히 그 상태로 만들었고, PR/MR target
// 브랜치 가드가 조용히 사라진 뒤 잘못된 타겟의 MR이 실제로 열렸다.
//
// deadcode는 main에서 도달 가능한 그래프만 분석하므로 고아 패키지를 아예
// 보지 못한다. 그래서 import 그래프를 직접 본다.
func TestProductionPackagesHaveImporters(t *testing.T) {
	packages := loadModulePackages(t)

	importers := map[string][]string{}
	for _, pkg := range packages {
		for _, imported := range pkg.allImports() {
			if !strings.HasPrefix(imported, modulePrefix) {
				continue
			}
			target := normalizeImport(imported)
			importers[target] = append(importers[target], normalizeImport(pkg.ImportPath))
		}
	}

	var orphans []string
	for _, pkg := range packages {
		path := normalizeImport(pkg.ImportPath)
		if pkg.Name == "main" {
			continue
		}
		// 프로덕션 파일이 없으면 테스트 전용 패키지다. 런타임에서 도달할
		// 대상이 애초에 없으므로 이 규칙의 관심 밖이다.
		if len(pkg.GoFiles) == 0 {
			continue
		}
		if len(importers[path]) > 0 {
			continue
		}
		if _, ok := orphanPackageAllowlist[path]; ok {
			continue
		}
		orphans = append(orphans, path)
	}
	sort.Strings(orphans)

	if len(orphans) > 0 {
		t.Fatalf("프로덕션 패키지가 모듈 안에서 import되지 않는다(유령 가드 위험): %s\n"+
			"배선을 되살리거나 패키지를 지운다. 테스트 지원 전용이라면 이유와 함께 orphanPackageAllowlist에 넣는다.",
			strings.Join(orphans, ", "))
	}
}

// TestOrphanPackageAllowlistHasNoStaleEntries는 allowlist가 실제로 고아인
// 패키지만 담도록 강제한다. 배선이 되살아난 뒤 남은 항목은 다음 고아를
// 가려 주므로 규칙을 무디게 만든다.
func TestOrphanPackageAllowlistHasNoStaleEntries(t *testing.T) {
	packages := loadModulePackages(t)

	byPath := map[string]modulePackage{}
	importCount := map[string]int{}
	for _, pkg := range packages {
		byPath[normalizeImport(pkg.ImportPath)] = pkg
		for _, imported := range pkg.allImports() {
			if strings.HasPrefix(imported, modulePrefix) {
				importCount[normalizeImport(imported)]++
			}
		}
	}

	for path, reason := range orphanPackageAllowlist {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("allowlist 항목 %s에 이유가 없다", path)
		}
		pkg, ok := byPath[path]
		if !ok {
			t.Errorf("allowlist 항목 %s가 더 이상 존재하지 않는다; 항목을 지운다", path)
			continue
		}
		if len(pkg.GoFiles) == 0 {
			t.Errorf("allowlist 항목 %s는 프로덕션 파일이 없어 규칙 대상이 아니다; 항목을 지운다", path)
		}
		if importCount[path] > 0 {
			t.Errorf("allowlist 항목 %s는 이제 import된다; 항목을 지운다", path)
		}
	}
}

const modulePrefix = "agent-harness/"

type modulePackage struct {
	ImportPath   string
	Name         string
	GoFiles      []string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func (p modulePackage) allImports() []string {
	all := make([]string, 0, len(p.Imports)+len(p.TestImports)+len(p.XTestImports))
	all = append(all, p.Imports...)
	all = append(all, p.TestImports...)
	all = append(all, p.XTestImports...)
	return all
}

func loadModulePackages(t *testing.T) []modulePackage {
	t.Helper()
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = findRepoRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -json ./...: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []modulePackage
	for decoder.More() {
		var pkg modulePackage
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list package: %v", err)
		}
		if !strings.HasPrefix(pkg.ImportPath, modulePrefix) {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages
}
