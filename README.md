# Go Docker Backend

Docker Desktop과 연동되는 간단한 컨테이너 관리 REST 백엔드입니다. Windows/macOS/Linux에서 동작하며, 프론트엔드 대시보드가 버튼으로 컨테이너 생성/시작/중지/삭제를 호출할 수 있도록 합니다.

## 요구사항
- Go 1.22+
- Docker Desktop (또는 Docker Engine)

## 설치

```bash
cd go-backend
go mod tidy
go run .
```

서버는 기본적으로 포트 `8081`에서 실행됩니다. 환경변수 `PORT`로 변경할 수 있습니다.

프론트엔드가 `http://localhost:3000`에서 실행 중이라면 `frontend/src/setupProxy.js`가 `/go` 경로를 `http://localhost:8081`로 프록시합니다.

## 엔드포인트

- GET `/go/containers?all=true` — 컨테이너 목록 조회 (중지 포함하려면 `all=true`)
- POST `/go/containers` — 컨테이너 생성
  - Body 예시:
    ```json
    { "image": "nginx:latest", "name": "my-nginx", "cmd": [], "env": [] }
    ```
- POST `/go/containers/{id}/start` — 컨테이너 시작
- POST `/go/containers/{id}/stop` — 컨테이너 중지
- DELETE `/go/containers/{id}` — 컨테이너 삭제 (강제 제거)
- POST `/go/containers/prune` — 중지된 컨테이너 정리

## cURL 예시

```bash
# 목록 (실행 중만)
curl http://localhost:8081/go/containers

# 목록 (모두)
curl "http://localhost:8081/go/containers?all=true"

# 생성
curl -X POST http://localhost:8081/go/containers \
  -H "Content-Type: application/json" \
  -d '{"image":"nginx:latest","name":"my-nginx"}'

# 시작
curl -X POST http://localhost:8081/go/containers/<id>/start

# 중지
curl -X POST http://localhost:8081/go/containers/<id>/stop

# 삭제
curl -X DELETE http://localhost:8081/go/containers/<id>

# 프룬
curl -X POST http://localhost:8081/go/containers/prune
```

## Windows (Docker Desktop)

- 별도 설정 없이 동작합니다. CLI는 `DOCKER_HOST`/`DOCKER_CONTEXT` 환경변수를 자동 인식합니다.
- 권한 문제 발생 시 Docker Desktop이 실행 중인지 확인하세요.





