# Visualization decision matrix

`diagram-design` 스킬의 형태(form)와 매체(medium) 판단 근거 카탈로그. 판단 로직은 SKILL.md가 소유하고, 이 문서는 그 판단이 놓이는 사실(렌더링 표면, mermaid 계열, 무의존 차트 경로)을 소유한다.

## 렌더링 표면

### Git 호스트 markdown (GitHub / GitLab)

- ` ```mermaid ` 펜스를 네이티브로 렌더링한다. 저장소 문서, README, 이슈 본문, PR/MR 본문이 모두 대상이다.
- 호스트가 묶은 mermaid 버전은 문서화되어 있지 않다. 안정 계열만 확실하게 렌더링된다고 보고, 신규 계열은 검증 없이 신뢰하지 않는다.
- 표, 헤딩, 코드 펜스, 인용은 GFM으로 렌더링된다.

### 터미널

- GFM 표와 코드 펜스는 구조적으로 표현 가능한 의존 가능한 시각화다.
- mermaid 펜스는 그림이 아니라 소스 텍스트로 보인다. 터미널 mermaid는 다른 곳에서 렌더링할 수 있는 이식 가능한 소스로 취급한다.
- 수량의 축약 표현은 코드 펜스 안 Unicode bar(`█▉▊…`)와 sparkline(`▁▂▃▄▅▆▇█`)으로 만든다.

### Self-contained HTML

- 렌더러를 페이지 안에 인라인으로 포함해야 한다. CDN에서 스크립트를 로드하는 페이지는 오프라인에서 깨지고 외부 요청이 발생하므로 만들지 않는다.
- 파일 위치는 플랫폼 임시 디렉터리다. 소비자 저장소 트리에 쓰지 않는다. BSD mktemp(macOS)는 뒤에 붙은 X만 치환하므로 `mktemp -d "${TMPDIR:-/tmp}/diagram-XXXXXX"`처럼 X를 템플릿 끝에 둔다.
- 파일당 한 페이지, 경로는 절대 경로로 보고하고 사용자가 열 수 있도록 남긴다.

### 금지 표면

- 데이터를 서드파티 클라우드로 egress하는 차트 MCP/서비스.
- CDN 의존 HTML.
- 검증되지 않은 신규 mermaid 계열을 Git 호스트 렌더링이 보장된다고 가정하는 것.

## mermaid 안정 계열 13종

| 계열 | 용도 |
|---|---|
| `flowchart` | 프로세스, 분기, 일반 노드-에지 관계 |
| `sequence` | 참가자 간 시간순 상호작용 |
| `class` | 타입, 필드, 관계(UML class) |
| `state` | 상태 머신과 전이 |
| `er` | 엔티티-관계 데이터 모델 |
| `gantt` | 기간이 있는 일정·작업 타임라인 |
| `pie` | 소수 조각의 부분-전체 비율 |
| `gitGraph` | 브랜치/커밋/머지 이력 |
| `mindmap` | 계층적 주제 분해 |
| `timeline` | 단일 축 연대기 |
| `quadrantChart` | 두 축 위 사분면 배치 |
| `requirement` | 요구 사항과 검증 관계 |
| `journey` | 만족도 점수가 있는 사용자 여정 |

신규 계열(`sankey`, `xychart`, `kanban`, `radar`, `treemap`)은 UNVERIFIED다. 쓰려면 먼저 임시 산출물로 렌더링을 검증한다.

## 무의존 차트 경로

페이지 위 차트는 외부 차트 라이브러리 없이 hand-authored 인라인 **SVG + CSS** 프리미티브(bar, line, scatter, area, stat tile)로 만든다. 터미널 위 차트는 monospace code fence 안의 Unicode 근사다.

## 검증 명령

```bash
# 문법 + 렌더링 검증 (네트워크 가능 환경)
npx -y @mermaid-js/mermaid-cli mmdc --input diagram.mmd --output diagram.svg
```

mmdc를 쓸 수 없으면 괄호·따옴표 균형, 예약어 충돌(`end`, `graph`, `subgraph`를 소문자 ID로 사용), 에지 화살표 문법을 수동 대조하고, 산출물에 검증 상태를 명시한다.

## 출처와 검증

- 형태+매체 라우팅 구조: melodic-software/claude-code-plugins `visualize` skill 및 `context/decision-matrix.md` (2026-08-26 확인).
- mermaid 계열 분류: https://mermaid.js.org/intro/
- Git 호스트 mermaid 렌더링: GitHub Docs "About writing and formatting on GitHub", GitLab Markdown 문서.
- 본 저장소 적합화: 매체 등급을 Git-host markdown / 터미널 / self-contained HTML로 재구성하고, 품질 규칙(노드 상한, CJK 라벨 quoting, classDef 예산, 검증 루프)을 추가했다.
