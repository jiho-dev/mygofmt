---
name: mygofmt
description: mygofmt의 Go 소스 포매팅 사용법. 파일·디렉터리 지정, 직접 수정, stdout 출력, 규칙별 on/off 설정 및 포매팅 검사를 안내하거나 실행할 때 사용한다.
---

# mygofmt 사용법

```bash
mygofmt [옵션] [파일 또는 디렉터리 ...]
```

아래 예시는 `mygofmt`이 PATH에 있는 경우다. 그렇지 않으면 `mygofmt` 대신 실행 파일 경로를 사용한다. 파일·디렉터리는 현재 작업 디렉터리 기준이며, 옵션은 경로 앞에 지정한다.

## 단일 파일 지정

```bash
mygofmt file.go
```

포매팅 결과를 stdout에 출력한다. 원본 파일은 수정하지 않는다.

## 여러 파일 지정

```bash
mygofmt first.go second.go third.go
```

각 파일의 포매팅 결과를 stdout에 이어서 출력한다.

## 디렉터리 지정

```bash
# 하나의 디렉터리
mygofmt ./pkg/example

# 여러 디렉터리
mygofmt ./pkg/example ./pkg/other

# 파일과 디렉터리를 함께 지정
mygofmt file.go ./pkg/example
```

하위 디렉터리의 `.go` 파일까지 처리한다. 순회 중 숨김 디렉터리, `vendor`, `testdata`는 제외한다. 제외 대상도 경로를 직접 지정하면 처리한다. 디렉터리 순회 중 심볼릭 링크는 건너뛴다. 링크를 직접 지정하면 오류다.

## 파일 직접 수정: -w

```bash
# 단일 파일 수정
mygofmt -w file.go

# 여러 파일 수정
mygofmt -w first.go second.go third.go

# 디렉터리 내 파일 수정
mygofmt -w ./pkg/example

# 여러 디렉터리 내 파일 수정
mygofmt -w ./pkg/example ./pkg/other
```

`-w`는 모든 결과를 임시 파일에 준비한 뒤 원본을 파일별로 교체한다. 준비 단계의 오류는 원본을 수정하지 않는다. 교체 단계의 오류는 앞서 교체한 파일을 되돌리지 않는다. 일반 권한 비트는 보존하지만 하드 링크 연결과 소유자·ACL 같은 추가 메타데이터는 보존하지 않는다.

## stdout 출력 및 별도 파일 저장

```bash
# 파일을 읽어 stdout 출력
mygofmt file.go

# 표준 입력을 읽어 stdout 출력
mygofmt < file.go

# 결과를 다른 파일에 저장
mygofmt file.go > formatted.go
```

여러 파일의 stdout 출력에는 파일 구분이 없다. 파일별로 출력하므로 뒤의 파일에서 오류가 나도 앞의 출력은 남는다. 입력 파일과 같은 파일에 `>`로 저장하지 않는다. 원본을 수정하려면 `-w`를 사용한다. 표준 입력에는 `-w`를 사용할 수 없다.

## 포매팅 검사

```bash
# 변경이 필요한 파일 목록 출력
mygofmt -l ./pkg/example

# 파일 수정 없이 검사: CI에서 사용
mygofmt -check ./pkg/example
```

`-check` 종료 코드는 정상 `0`, 포매팅 변경 필요 `1`, 오류 `2`다. `-w`와 `-check`는 함께 사용할 수 없다.

## 소스 주석으로 포매팅 제외

지시문은 각각 독립된 주석 줄에 쓴다.

```go
// mygofmt:ignore-file
package example
```

`ignore-file`은 `package` 앞에 두며 파일 전체를 원문 그대로 유지한다. 다른 mygofmt 지시문과 함께 사용할 수 없다.

```go
func example() {
	// mygofmt:ignore
	value := []int{1,2,3} // 바로 다음 한 줄만 유지

	// mygofmt:off
	if ready { run() } // 두 지시문 사이를 유지
	// mygofmt:on

	finish() // 이 줄은 정상 포매팅
}
```

`ignore` 다음에는 비어 있지 않은 소스 줄이 필요하다. `off`와 `on`은 짝을 맞추고 중첩하지 않는다. `ignore` 대상 줄에서 시작한 원시 문자열이나 블록 주석이 여러 줄에 걸치면, 해당 토큰이 끝나는 줄까지 원문을 유지한다. 제외한 코드도 유효한 Go 구문이어야 한다. `mygofmt -check file.go`는 제외 구간의 원래 배치를 허용하고 나머지 부분만 검사한다.

## 설정 파일 지정

기본 설정 파일은 `~/.config/mygofmt.yaml`이다. 파일이 없으면 내장 기본값을 사용한다. `base`에서 엔진을, `layout`에서 수치를, `rules`에서 기능별 on/off를 설정한다. `calls`는 특정 호출의 빈 줄 위치를, `files`는 생성 파일과 `//line` 파일의 처리 방식을 정한다. 전체 설정은 [예시 YAML](references/mygofmt.yaml.example), 규칙별 이름·기본값·예제는 [규칙 안내](references/rules.md)에 있다.

설정 스키마 버전은 `1`이며, 생략한 규칙은 현재 기본값을 사용한다. 모든 규칙이 기본 `true`인 것은 아니다. 기존 YAML의 `layout.wrap`, `layout.defer-function-body`, `whitespace`, `base.gofumpt.extra` 키는 `rules`로 옮겨야 한다.

```yaml
version: 1
rules:
  wrap-call-arguments: true
  blank-before-return: false
```

```bash
# 기본 설정 사용
mygofmt -w file.go

# 다른 설정 파일 사용
mygofmt -config ./custom-format.yaml -w file.go

# 설정의 줄 길이를 CLI 옵션으로 덮어쓰기
mygofmt -m 100 -w file.go
```

명시한 CLI 옵션이 YAML 설정보다 우선한다. 잘못된 설정이나 읽을 수 없는 설정 파일은 오류다. `-config`로 지정한 파일이 없으면 오류다.

## gofumpt 함께 사용

설정 파일의 `base` 항목을 다음과 같이 지정한다. 추가 gofumpt 규칙도 `rules` 아래에서 켜며, 기본값은 `false`다.

```yaml
base:
  formatter: gofumpt
  allow-syntax-changes: true
rules:
  gofumpt-group-params: true
```

이 내용을 `~/.config/mygofmt.yaml`에 반영하면 별도 `-config` 옵션 없이 적용된다.

```bash
mygofmt -w file.go
mygofmt -check file.go
```

## 옵션 확인

```bash
mygofmt -h
```

| 옵션 | 용도 |
|---|---|
| `-w` | 원본 파일에 저장 |
| `-l` | 변경이 필요한 파일 목록 출력 |
| `-check` | 변경 필요 여부를 종료 코드로 반환 |
| `-config <경로>` | 설정 파일 선택 |
| `-m <숫자>` | 선호 최대 줄 길이 지정 |
| `-tab-width <숫자>` | 줄 길이 계산에 사용할 탭 너비 지정 |
| `-log-methods Info,Infof,EndError` | 빈 줄 규칙을 적용할 로깅 메서드 지정 |
| `-h` | 도움말 출력 |
