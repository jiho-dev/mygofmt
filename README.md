# mygofmt

Go 소스의 줄바꿈과 빈 줄을 설정에 따라 정리하는 포매터다. 기본 엔진은 `gofmt`이며, 설정에서 `gofumpt`를 선택할 수 있다. 이 도구의 추가 규칙은 소스의 AST를 바꾸지 않는다. gofumpt 모드에서는 upstream의 구문 단순화가 적용될 수 있다.

## 빌드와 사용

Go 1.25.4 이상이 필요하다. 저장소 루트에서 실행한다.

```bash
go build -buildvcs=false -o mygofmt .
./mygofmt file.go                 # 결과를 stdout에 출력
./mygofmt -w ./pkg                # 파일에 적용
./mygofmt -l ./pkg                # 변경이 필요한 파일 목록
./mygofmt -check ./pkg            # 변경이 필요하면 종료 코드 1
./mygofmt < file.go               # 표준 입력
```

옵션은 경로 앞에 둔다. `-config`를 생략하면 `~/.config/mygofmt.yaml`을 읽고, 파일이 없으면 내장 기본값을 사용한다. 입력·설정 오류의 종료 코드는 2다. `-w`와 `-check`는 함께 사용할 수 없다.

## 설계와 처리 흐름

설정은 다섯 부분으로 나뉜다. `base`는 gofmt/gofumpt 엔진과 언어 버전을 고르고, `layout`은 줄 길이·탭 너비·호출 인자 수 같은 수치를 정한다. `rules`에는 각 포매팅 기능의 고유 이름과 `true`/`false` 스위치를 둔다. `calls`는 로그 같은 호출의 메서드·수신자 목록과 빈 줄 위치를 지정한다. `files`는 생성 파일과 `//line` 지시문 파일의 처리 방식을 정한다.

처리 순서는 입력 구문 확인 → 파일 정책·소스 주석 예외 확인 → 기본 엔진 → 활성화된 추가 규칙 → gofmt 출력이다. 결과가 안정될 때까지 반복하고, 안정되지 않으면 오류를 반환한다. 파일을 저장할 때는 모든 입력을 먼저 포매팅하여 구문 오류로 일부만 저장되는 일을 방지한다.

설정 스키마 버전은 `1`이다. 기존 기본값을 유지한다. 예를 들어 함수 매개변수와 호출 인자 자동 줄바꿈은 기본 `false`, 복합 리터럴 줄바꿈과 반환문 앞 빈 줄은 기본 `true`다. 새 YAML에서는 모든 기능 스위치를 `rules:` 아래에 둔다. 이전 `layout.wrap`, `layout.defer-function-body`, `whitespace`, `base.gofumpt.extra` 키는 사용할 수 없다.

전체 설정은 [예시 YAML](mygofmt.yaml.example), **기능별 이름·기본값·설명·입출력 예제는 [규칙 안내](docs/rules.md)**를 참조한다. 생략한 키는 내장 기본값을 사용하며, 알 수 없는 키·중복 키·`null`·잘못된 값·여러 YAML 문서는 오류다.

## 소스 주석 예외

아래 지시문은 독립된 줄에 쓴다. `ignore-file`은 `package` 앞에 단독으로 두고 파일 전체를 유지한다. `ignore`는 다음 한 줄, `off`와 `on`은 둘 사이의 소스를 유지한다. 제외한 코드도 유효한 Go 구문이어야 한다.

```go
// mygofmt:ignore-file
package example
```

```go
func example() {
	// mygofmt:ignore
	value := []int{1,2,3}
	// mygofmt:off
	if ready { run() }
	// mygofmt:on
	_ = value
}
```

## 검증

```bash
go test ./... -count=1
go vet ./...
```

초기 검증 기록은 [VALIDATION.md](VALIDATION.md)에 있다.
