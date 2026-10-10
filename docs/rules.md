# 포매팅 규칙

각 기능은 `rules:` 아래의 이름으로 켜거나 끈다. 생략하면 표의 기본값을 사용한다. `true`는 해당 규칙을 적용하고 `false`는 적용하지 않는다. 예제에서 `⏎`는 줄바꿈, `⏎⏎`는 빈 줄 하나를 뜻한다. 표의 예제는 해당 규칙으로 달라지는 부분만 보여 준다. gofmt의 기본 정렬은 항상 적용되며, `base.formatter: gofumpt`로 선택한 upstream 기본 규칙은 개별적으로 끌 수 없다. 생성 파일과 `//line` 파일은 `files` 정책이 우선한다.

```yaml
version: 1
layout:
  max-line-length: 120
  tab-width: 4
  min-call-arguments: 2
rules:
  wrap-call-arguments: true
  blank-before-return: false
```

## 줄바꿈

| 이름 | 기본값 | 기능과 예제 (`입력` → `true`일 때) |
|---|---|---|
| `wrap-function-parameters` | `false` | 길이가 넘는 함수 매개변수를 나눈다. `func f(first VeryLongType, second VeryLongType)` → `func f(⏎ first VeryLongType,⏎ second VeryLongType,⏎)` |
| `wrap-function-results` | `false` | 길이가 넘는 함수 결과 목록을 나눈다. `func f() (FirstLongResult, SecondLongResult)` → `func f() (⏎ FirstLongResult,⏎ SecondLongResult,⏎)` |
| `wrap-type-parameters` | `false` | 길이가 넘는 함수·타입 선언(타입 별칭 포함)의 타입 매개변수 목록을 나눈다. `func f[First VeryLongConstraint, Second VeryLongConstraint]()` → `func f[⏎ First VeryLongConstraint,⏎ Second VeryLongConstraint,⏎]()` |
| `wrap-call-arguments` | `false` | `layout.min-call-arguments` 이상인 긴 호출을 나눈다. `send(longFirstArgument, longSecondArgument)` → `send(⏎ longFirstArgument,⏎ longSecondArgument,⏎)` |
| `wrap-composite-literals` | `true` | 긴 복합 리터럴의 항목을 나눈다. `Result{First: longFirstValue, Second: longSecondValue}` → `Result{⏎ First: longFirstValue,⏎ Second: longSecondValue,⏎}` |
| `wrap-nested-composite-literals` | `true` | 여러 줄 키 기반 리터럴 안의 키 기반 자식 리터럴도 나눈다. `Outer{⏎ Inner: Inner{A: a, B: b},⏎}` → `Outer{⏎ Inner: Inner{⏎ A: a,⏎ B: b,⏎ },⏎}`. `wrap-composite-literals`가 켜져 있어야 한다. |
| `balance-logical-conditions` | `true` | 긴 `&&`·`||` 조건식을 연산자 경계에서 나눈다. `if longFirstCondition && longSecondCondition` → `if longFirstCondition &&⏎ longSecondCondition` |
| `multiline-defer-body` | `true` | `defer` 익명 함수의 한 줄 본문을 여러 줄로 편다. `defer func() { cleanup() }()` → `defer func() {⏎ cleanup()⏎ }()` |

줄 길이는 `layout.max-line-length`의 선호 기준이다. 긴 문자열·주석·단일 식별자는 자르지 않는다. `layout.tab-width`는 화면 열 너비 계산에 사용한다. 이미 작성된 줄바꿈은 가능한 한 보존한다.

## 빈 줄과 호출

| 이름 | 기본값 | 기능과 예제 (`입력` → `true`일 때) |
|---|---|---|
| `preserve-existing-blank-lines` | `true` | 작성자가 넣은 문장 사이 빈 줄을 유지한다. `first()⏎⏎second()` → 그대로 유지한다. `false`면 주석 없는 문장 사이 여분의 빈 줄을 정리한다. |
| `blank-after-multiline-signature` | `true` | 함수·메서드·익명 함수의 시그니처가 두 줄 이상이면 본문 첫 문장 또는 선행 주석 앞에 빈 줄을 둔다. 빈 본문과 한 줄 시그니처에는 추가하지 않는다. 단일 문장 블록의 빈 줄 제거보다 우선한다. |
| `blank-before-return` | `true` | 앞 문장이 있는 반환문 앞에 빈 줄을 둔다. `work()⏎return nil` → `work()⏎⏎return nil` |
| `blank-before-loop` | `true` | 반복문 앞에 빈 줄을 둔다. `prepare()⏎for ready { run() }` → `prepare()⏎⏎for ready { run() }` |
| `blank-after-loop` | `true` | 반복문 뒤에 빈 줄을 둔다. `for ready { run() }⏎finish()` → `for ready { run() }⏎⏎finish()` |
| `blank-after-control-block` | `true` | `if`·`switch`·`select` 뒤에 빈 줄을 둔다. `if ok { run() }⏎finish()` → `if ok { run() }⏎⏎finish()`. 반복문은 별도 규칙이다. |
| `blank-after-multiline-initialization` | `true` | 여러 줄 복합 리터럴 대입 뒤에 빈 줄을 둔다. `value := Result{⏎ A: a,⏎}⏎use(value)` → `value := Result{⏎ A: a,⏎}⏎⏎use(value)` |
| `join-call-error-checks` | `true` | 호출 결과 대입과 바로 뒤의 단순 `err != nil` 검사를 붙인다. `value, err := load()⏎⏎if err != nil { return err }` → 두 문장 사이 빈 줄을 제거한다. 주석이 끼면 유지한다. |
| `space-configured-calls` | `true` | `calls`에 지정한 호출 앞뒤에 빈 줄을 둔다. `prepare()⏎log.Info("ready")⏎finish()` → `prepare()⏎⏎log.Info("ready")⏎⏎finish()` |
| `compact-single-statement-blocks` | `true` | 문장이 하나인 블록의 중괄호 안쪽 빈 줄을 제거한다. `if ok {⏎⏎run()⏎⏎}` → `if ok {⏎run()⏎}` |

`calls`의 `blank-before`, `blank-after`는 **문장 사이 호출 빈 줄의 적용 위치**를 정한다. 블록이나 `case`의 첫 호출 앞과 마지막 호출 뒤에는 설정과 관계없이 호출용 빈 줄을 추가하지 않는다. `include-block-boundaries`는 기존 설정 호환을 위해 허용하지만 동작에는 영향을 주지 않는다. 여러 줄 함수 시그니처 뒤 빈 줄은 별도의 `blank-after-multiline-signature` 규칙이 적용된다. `space-configured-calls: false`면 호출 규칙의 빈 줄 추가 전체가 꺼진다. 메서드·수신자 목록은 호출을 찾는 조건이며, `calls: []`도 호출 빈 줄을 끈다.

`space-configured-calls`가 켜져 있으면 대상 호출 바로 위의 독립된 주석 그룹과 호출 사이 빈 줄을 제거한다. `preserve-existing-blank-lines: true`여도 이 간격은 정리하며, 호출 앞 빈 줄이 필요하면 주석 앞에 둔다. 이전 코드 줄 끝의 주석과 더 앞쪽 주석 문단의 간격은 유지한다.


```yaml
calls:
  - methods: [Info, Infof]
    receivers: [log, logger]
    blank-before: true
    blank-after: true
    include-block-boundaries: false
```

## gofumpt 결과 보존과 추가 규칙

아래 규칙은 `base.formatter: gofumpt`일 때 적용된다. gofumpt를 선택하려면 `base.allow-syntax-changes: true`도 필요하다.

| 이름 | 기본값 | 기능과 예제 (`입력` → `true`일 때) |
|---|---|---|
| `preserve-block-comment-gaps` | `true` | gofumpt가 줄인 블록 주석 양옆의 원래 빈 줄을 복원한다. `work()⏎⏎/* 설명 */⏎⏎finish()` → 두 빈 줄을 유지한다. |
| `preserve-signature-layout` | `true` | 토큰이 같은 함수 시그니처와 타입 선언의 타입 매개변수에서 gofumpt가 바꾼 원래 줄 배치를 복원한다. `func f(⏎ a int, b int) {}` → 매개변수 줄 배치를 유지한다. 의도적으로 켠 `wrap-function-*`·`wrap-type-parameters` 규칙은 우선한다. |
| `gofumpt-group-params` | `false` | 같은 타입의 연속 매개변수를 묶는다. `func f(a int, b int)` → `func f(a, b int)` |
| `gofumpt-clothe-returns` | `false` | 이름 붙은 결과를 가진 함수의 빈 `return`에 결과 값을 적는다. `func f() (n int) { n = 1; return }` → `func f() (n int) { n = 1; return n }` |
| `gofumpt-balance-calls` | `false` | 여러 줄 호출에서 여는 괄호가 줄 끝에 있으면 닫는 괄호를 별도 줄에 둔다. `send(⏎ value)` → `send(⏎ value⏎)` |

## 규칙이 아닌 설정

`base.formatter`는 기본 엔진 선택, `base.gofumpt.lang-version`과 `module-path`는 gofumpt 옵션이다. `files.generated`와 `files.line-directives`는 각 파일에서 `gofmt-only` 또는 `skip`을 선택한다. `// mygofmt:ignore-file`, `ignore`, `off`, `on`은 소스에 작성하는 예외 지시문이다. 이 항목들은 개별 포매팅 규칙 스위치가 아니다.
