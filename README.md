# mygofmt

소스 배치 규칙을 설정으로 적용하는 Go 포매터다. gofmt는 표준 라이브러리, gofumpt는 수정하지 않은 upstream 라이브러리로 재사용한다. 추가 규칙은 YAML로 선택한다.

이 README는 현재 **사용법과 설정 항목**의 안내 문서다. 합의한 포매팅 범위와 설계 이유는 [설계 및 합의 사항](2026-10-08-mygofmt-design.md), 배포할 설정 값은 [mygofmt.yaml.example](mygofmt.yaml.example)을 기준으로 한다. 개인 실행 시에는 `~/.config/mygofmt.yaml`을 읽는다.

## 빌드와 실행

독립된 Go 모듈이다. 빌드에 **Go 1.25.4 이상**이 필요하다. 다음 명령은 이 저장소의 루트에서 실행한다.

```bash
go build -buildvcs=false -o /tmp/mygofmt .

# 결과 출력. 원본을 수정하지 않는다.
/tmp/mygofmt path/to/file.go

# 파일 또는 디렉터리의 소스에 적용
/tmp/mygofmt -w pkg/example

# 필요한 변경 목록 출력
/tmp/mygofmt -l pkg/example

# CI 검사
/tmp/mygofmt -check pkg/example

# 표준 입력
/tmp/mygofmt < path/to/file.go

# 명시 CLI 옵션은 YAML보다 우선
/tmp/mygofmt -m 100 -tab-width 4 -log-methods Info,Infof,EndError path/to/file.go
```

옵션은 경로 인자 앞에 둔다. `-config`를 생략하면 `~/.config/mygofmt.yaml`을 읽는다. 이 파일이 없으면 내장 기본 설정을 사용한다. 명시한 `-config` 경로가 우선하며, 기본 파일이 잘못되었거나 읽을 수 없으면 오류를 반환한다. `XDG_CONFIG_HOME`과 무관하게 요청한 홈 디렉터리 경로를 사용한다. `-h`로 전체 옵션을 확인한다.

바이너리 종료 코드는 정상 0, `-check`에서 변경 필요 1, 입력·설정·구문·실행 오류 2다. `-check`는 전체 조합 결과를 원본과 비교하며 파일을 수정하지 않는다. `-w`와 `-check`는 함께 사용할 수 없다.

## 소스 안에서 포매팅 제외

주석 지시문은 설정 파일 변경 없이 예외를 지정한다. 각 지시문은 **독립된 줄**에 쓴다.

```go
// mygofmt:ignore-file
package example
```

`ignore-file`은 `package` 앞에서 파일 전체를 원문 그대로 유지한다. 파일에서 유일한 mygofmt 지시문이어야 한다.

```go
func example() {
	// mygofmt:ignore
	value := []int{1,2,3} // 바로 다음 물리적 한 줄을 유지

	// mygofmt:off
	if ready { run() } // 이 구간은 원문 그대로 유지
	// mygofmt:on

	finish() // 구간 밖은 포매팅
}
```

`ignore`는 바로 다음의 비어 있지 않은 한 줄을, `off`·`on`은 두 지시문 **사이**의 소스를 그대로 유지한다. 지시문이 들어 있는 줄은 제외 구간에 포함되지 않는다. 짝이 없는 `off`·`on`, 중첩 구간, 구간 안의 `ignore`, 알 수 없는 지시문은 오류다. 문자열 안의 같은 문구는 지시문이 아니다. 제외된 코드도 유효한 Go 구문이어야 한다. `-check`는 제외 구간의 기존 배치를 변경 필요로 보고하지 않으며, 나머지 소스는 검사한다.

## 설정

[mygofmt.yaml.example](mygofmt.yaml.example)은 gofumpt와 소스 배치 규칙을 담은 예시 설정이다. 실제 사용자 설정은 `~/.config/mygofmt.yaml`에 둔다. 예시 파일은 자동으로 읽히지 않으며, 홈 설정이 없는 환경은 아래 **내장 기본값**으로 동작한다. 설정을 바꿀 때는 홈 설정과 예시 파일을 함께 갱신한다. 생략한 키는 내장 기본값을 유지한다. `calls` 목록을 명시하면 기본 호출 규칙 목록을 대체한다. 알 수 없는 키, 중복 키·메서드, null, 잘못된 값과 여러 YAML 문서는 오류다.

| 설정 | 내장 기본값 | 의미 |
|---|---|---|
| `version` | `1` | 설정 스키마 버전 |
| `base.formatter` | `gofmt` | 기본 엔진. `gofumpt`도 선택 가능 |
| `base.allow-syntax-changes` | `false` | gofumpt 선택 시 `true`를 명시해야 함 |
| `base.gofumpt.lang-version` | `go1.25` | 포매팅 대상 언어 버전. 도구 빌드 버전과 별개 |
| `base.gofumpt.module-path` | 빈 문자열 | gofumpt의 import 분류에 사용할 대상 모듈 경로 |
| `base.gofumpt.extra.*` | 모두 `false` | `group-params`, `clothe-returns`, `balance-calls` |
| `layout.max-line-length` | `120` | 선호 최대 열 수. 최소 20 |
| `layout.tab-width` | `4` | 탭 하나의 계산 너비. 최소 1 |
| `layout.min-call-arguments` | `2` | 호출 줄바꿈을 켰을 때 적용할 최소 인자 수. 최소 1 |
| `layout.wrap.function-parameters`, `function-results`, `type-parameters` | 모두 `false` | 함수 시그니처 자동 줄바꿈. 기본적으로 제외 |
| `layout.wrap.call-arguments` | `false` | 함수 호출 인자 자동 줄바꿈. 기본적으로 제외 |
| `layout.wrap.composite-literals`, `logical-conditions` | 모두 `true` | 긴 초기화 줄바꿈·조건식 균형 배치 |
| `layout.wrap.nested-composite-literals` | `true` | 여러 줄 초기화 내부의 중첩 구조체·맵을 필드·키별로 줄바꿈 |
| `layout.defer-function-body` | `multiline` | defer 익명 함수 본문을 여러 줄로. `preserve`도 지원 |
| `whitespace.preserve-existing` | `true` | 기본 엔진 이후의 기존 빈 줄 보존. 단일 문장 블록의 중괄호 안쪽은 예외 |
| `whitespace.before-return` | `true` | 앞 문장이 있는 반환문 앞 빈 줄 |
| `whitespace.before-loop`, `after-loop` | `true` | 반복문 앞뒤 빈 줄 |
| `whitespace.after-control-block` | `true` | if/switch/select 등 뒤 빈 줄. 반복문은 별도 설정 |
| `whitespace.after-multiline-initialization` | `true` | 여러 줄 구조체·컬렉션 대입 뒤 빈 줄 |
| `whitespace.call-error-check` | `adjacent` | 호출 대입과 단순 `err != nil` 검사 붙이기. `preserve`도 지원 |
| `files.generated`, `line-directives` | `gofmt-only` | 특수 파일은 gofmt만 적용. `skip`은 원문 유지 |

호출 규칙은 사용자가 Go 코드를 작성하지 않고 추가할 수 있다. 현재 배포용 설정의 로그 규칙은 다음과 같다.

```yaml
calls:
  - methods: [Trace, Tracef, Debug, Debugf, Info, Infof, Warn, Warnf, Warning, Warningf, Error, Errorf, Fatal, Fatalf, Panic, Panicf, Print, Printf, Println, End, EndElapsed, EndError]
    receivers: [log, logger, fl, s.logger]
    blank-before: true
    blank-after: true
    include-block-boundaries: false
```

`calls: []`는 호출 특별 처리를 끈다. `include-block-boundaries`는 함수·조건문·반복문·defer 블록 및 switch/select case의 시작·끝을 포함할지 정한다. `receivers`를 지정하면 `fl`, `s.logger` 같은 소스의 수신자 이름과 메서드 이름을 함께 판별한다. 수신자는 식별자 또는 점으로 연결한 필드 이름이며 타입 추론이나 와일드카드는 사용하지 않는다. 생략하거나 빈 목록이면 모든 수신자가 대상이다. 대입이나 표현식 내부의 호출에는 적용하지 않는다.

문장이 하나뿐인 `{}` 블록은 중괄호와 문장 사이의 빈 줄을 추가하지 않으며, 주석이 없는 기존 빈 줄도 제거한다. 주석이 끼어 있는 구간은 보존한다. `preserve-existing: false`는 블록 내 문장 사이의 **주석 없는** 빈 줄을 정리한 뒤 설정된 빈 줄을 적용한다. raw string과 주석 본문을 지우지 않는다. true도 gofumpt가 앞 단계에서 제거한 원본 여백을 복원하지는 않는다.

## 처리와 범위

gofumpt가 함수 시그니처의 마지막 쉼표·닫는 괄호 배치를 바꿔도, 구문 토큰이 같은 목록은 원래 배치를 복원한다. 여러 줄 블록 주석 앞뒤의 기존 빈 줄도 보존한다.

입력 구문 확인 → 특수 파일 정책 → 기본 엔진 → 자체 줄바꿈·빈 줄 규칙 → gofmt 출력 순서다. 전체 결과가 안정될 때까지 최대 8회 반복하고 안정되지 않으면 저장하지 않는다. 최종 형식 검사는 gofumpt 단독 검사가 아닌 `mygofmt -check`로 한다.

기본 모드에서 추가 규칙은 소스 배치만 변경한다. 함수 분리, 상수 선정, 검증 흐름 변경, 지역 변수 추출·이름 변경, 다중 대입 분리, 콜백 변경은 수행하지 않는다. 같은 목적의 대입·역할 변경처럼 의미 판단이 필요한 구간은 작성자가 넣은 여백을 따른다.

**gofumpt 모드에는 upstream 기본 구문 단순화가 포함된다.** 특정 기본 규칙을 설정으로 끌 수는 없으며 자체 규칙을 끄더라도 gofumpt 동작은 남는다. 자체 규칙은 원본 gofumpt를 수정하거나 규칙을 주입하지 않고 공개 `format.Source` API의 결과에 적용한다. 의존성은 `go.mod`/`go.sum`에 고정한다.

함수 시그니처와 모든 함수 호출의 인자 목록은 기본적으로 자동 줄바꿈하지 않는다. `call-arguments: true`로 별도 활성화했을 때만 인자 수와 줄 길이 기준을 적용한다. 기존에 작성된 줄바꿈은 보존한다. 긴 외부 표현식을 먼저 나눈 뒤 내부 표현식의 길이를 다시 평가하므로 짧은 중첩 호출은 한 줄로 유지한다. 여러 줄로 작성된 키·값 초기화는 항목별로 나누며, `nested-composite-literals: true`는 그 안의 중첩 구조체·맵도 나눈다. 이 옵션은 `composite-literals: true`일 때 적용된다. 짧은 단일 줄 초기화, 빈 초기화와 짧은 위치 기반 값(예: `[]string{"host-a"}`)은 유지한다. 최대 길이는 선호 기준이다. 문자열·raw string·주석·단일 식별자를 쪼개거나 코드를 추출하지 않으므로 긴 줄이 남을 수 있다. 기존 구역 구분 주석은 보존하며 새 구역을 만들거나 함수를 재배치하지 않는다. 표준 gofmt의 import 정렬과 주석 정리는 적용한다.

생성 파일은 `// Code generated ... DO NOT EDIT.` 규약으로 판별한다. line 지시문은 AST 주석에서 확인하므로 raw string 안의 `//line`은 영향을 주지 않는다. 지시문 스타일 주석이 있는 파일은 추가 줄바꿈으로 위치 정보가 달라지는 것을 피하기 위해 기본적으로 gofmt만 적용한다.

디렉터리 순회는 숨김 디렉터리·`vendor`·`testdata`를 건너뛰며 직접 지정한 디렉터리는 처리한다. 디렉터리 순회 중 심볼릭 링크는 건너뛰며, 직접 지정한 링크는 오류다. 모든 입력을 포매팅한 뒤 저장하여 구문·포매팅 오류로 일부만 저장되는 일을 방지한다. 저장 중 I/O 오류가 발생하면 이미 저장한 파일은 되돌리지 않는다.

## 테스트

최초 구현 시점의 검증 결과는 [VALIDATION.md](VALIDATION.md)에 기록했다. 최신 코드는 아래 명령으로 다시 검증한다.

다음 명령은 저장소 루트에서 실행한다.

```bash
go test ./... -count=1
go test -run '^$' -fuzz '^FuzzFormat$' -fuzztime=10s -parallel=2
go test -run '^$' -fuzz '^FuzzConfiguredFormat$' -fuzztime=10s -parallel=2
go test -run '^$' -fuzz '^FuzzGofumptComposition$' -fuzztime=10s -parallel=2
go vet ./...
```

`testdata`의 22개 입력·기대 출력은 문법적으로 유효한 형식 검사용 소스이며 외부 타입·로거 구현은 포함하지 않는다. 기본 출력과 선택적으로 활성화한 시그니처·호출 규칙의 출력, 규칙별 on/off, 호출 전후·경계 옵션, YAML 오류·CLI 우선순위, 특수 파일, 실제 gofumpt와의 조합을 검사한다. 반복 실행 안정성, gofmt 호환성, 기본 모드 AST·주석 보존과 gofumpt 단계 이후 자체 규칙의 AST 보존도 검사한다. `//line` 샘플의 기대 출력은 보호 정책에 맞춰 gofmt만 적용한 결과로 갱신했다.

## 예시 설정 적용

`~/.config/mygofmt.yaml`에 gofumpt와 소스 배치 규칙을 함께 둔다. 로그 수신자를 제한하여 `require.Error`와 `t.Fatal` 같은 테스트 호출을 제외하고 블록 경계의 로그 빈 줄을 끈다. 사용자 설정의 `base.formatter: gofumpt`와 `base.allow-syntax-changes: true`가 gofumpt를 활성화한다.

```bash
# 이 저장소의 루트에서 실행
go build -buildvcs=false -o /tmp/mygofmt .
/tmp/mygofmt -w pkg/example
/tmp/mygofmt -check pkg/example
```

## 조건식 줄바꿈

`layout.wrap.logical-conditions: true`이면 `&&`·`||` 조건식의 연산자 뒤 공백을 정리한다. 길이 기준 안이면 한 줄, 길면 기준 안에서 두 줄 길이의 차이가 가장 작은 연산자 경계로 나눈다. 두 줄로 들어가지 않으면 연산자 경계에서 추가 줄바꿈한다. 기존에 부분적으로 나뉜 조건식도 같은 기준을 적용한다. 기준 길이는 `layout.max-line-length`이며 탭 너비도 반영한다.

괄호와 연산자 순서는 유지하며 조건식 안에 주석 또는 여러 줄 피연산자가 있으면 기존 배치를 유지한다. 나눌 수 없는 긴 단일 피연산자는 길이 기준을 넘을 수 있다. `logical-conditions: false`는 이 정리를 끈다.
