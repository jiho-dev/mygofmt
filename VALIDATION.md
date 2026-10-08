# mygofmt 최초 구현 당시 검증 기록

검증일: 2026-10-08. 아래 수치와 적용 범위는 당시 스냅샷이다. 현재 사용법과 설정은 [README.md](README.md) 및 [mygofmt.yaml.example](mygofmt.yaml.example)을 따른다.

## 범위

설정 기반 확장 구현과 gofumpt v0.11.0 연동을 검증했다. 검증 대상 애플리케이션 소스에는 포매터를 저장 모드로 적용하지 않았다.

## 결과

| 검사 | 결과 |
|---|---|
| Go 1.25.8에서 `go test ./... -count=1` | 통과 |
| Go 1.26.6에서 같은 테스트 | 통과 |
| 기존 입력·기대 출력 샘플 21개 | 통과. line 지시문 샘플은 gofmt-only 정책 반영 |
| 규칙별 on/off 및 호출 앞뒤·블록 경계 | 통과 |
| 잘못된 YAML, 중복·미지원 설정, CLI 우선순위 | 통과 |
| 생성 파일·line 지시문·원문 유지 정책 | 통과 |
| gofumpt 기본 규칙·extra 규칙·자체 규칙 조합 | 통과 |
| 전체 도구 패키지 커버리지 | 90.2% |
| 저장소 `.golangci.yaml`로 도구 모듈 lint | 0 issues |

Go 1.26.6에서 10초씩 수행한 퍼즈 검사:

- 기본 형식 `FuzzFormat`: 29,444회, 통과.
- 설정 조합 `FuzzConfiguredFormat`: 20,543회, 통과.
- gofumpt 조합 `FuzzGofumptComposition`: 69,544회, 통과.

## 재현

현재 저장소 루트에서 실행하는 명령:

```bash
go test ./... -count=1
go test ./... -coverpkg=./... -coverprofile=/tmp/mygofmt-coverage.out
go tool cover -func=/tmp/mygofmt-coverage.out
go vet ./...
```

퍼즈 명령은 [README](README.md#테스트)에 있다. 검증 환경에서는 쓰기 가능한 임시 경로를 Go·모듈·lint 캐시로 사용했다. 외부 타입이 포함된 형식 샘플은 컴파일 대신 구문·AST·출력 보존을 검사한다.
