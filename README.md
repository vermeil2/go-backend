## Go Docker Backend

Docker Desktop / Docker Engine과 연동되는 **도커 관리용 REST API 서버**입니다.  
특정 프론트엔드에 종속되지 않고, 어떤 클라이언트(React, Vue, 모바일, Postman 등)에서도 HTTP 호출만으로 컨테이너/이미지/볼륨/Compose를 제어할 수 있도록 설계되었습니다.

### 주요 기능

- **컨테이너 관리**
  - 컨테이너 목록 조회 (실행 중 / 전체)
  - 컨테이너 생성 / 시작 / 중지 / 삭제
  - 중지된 컨테이너 일괄 정리(prune)
  - 컨테이너 로그 조회, 통계(CPU/메모리 등) 조회, 컨테이너 내부 명령 실행

- **이미지 관리**
  - 로컬 도커 이미지 목록 조회
  - Dockerfile 내용을 바탕으로 이미지 빌드

- **볼륨 관리**
  - 볼륨 목록 조회 / 생성 / 삭제 / Prune
  - 볼륨 내부 파일 시스템 브라우징(ls -la 파싱)

---
## 컨테이너 관리 

![컨테이너_시연](컨테이너_시연영상.gif)

---
## 블록코딩 실습

![블록코딩 시연](블록코딩_시연영상.gif)

---

## 기술 스택 및 구조

- **언어 / 런타임**
  - Go 1.22+

- **외부 의존성**
  - Docker Desktop 또는 Docker Engine
  - `github.com/docker/docker` Go SDK
  - `github.com/gorilla/mux` (라우팅)

- **주요 디렉터리**
  - `main.go` / `routes.go`  
    - HTTP 서버 실행 및 라우팅 설정
  - `handlers/`  
    - `containers.go` : 컨테이너 관련 API  
    - `images.go` : 이미지 관련 API  
    - `volumes.go` : 볼륨 관련 API  
    - `compose.go` : docker-compose 관련 API  
    - `files.go` : compose/nginx 설정 파일 저장 API
  - `types/types.go`  
    - 공용 요청/응답 DTO 및 에러 응답 구조체
  - `utils/docker.go`  
    - `WriteJSON`, `NewDockerClient` 등 공용 헬퍼

---

### 요구 사항

- Go 1.22+
- Docker Desktop 또는 Docker Engine (로컬에서 `docker` 명령이 동작해야 함)

---

### 실행 방법

```bash
cd go-backend

# 의존성 정리
go mod tidy

# 서버 실행
go run .
```

- 기본 포트: `8081`
- 환경변수 `PORT`로 변경 가능

```bash
PORT=9090 go run .
```

---

### API 개요

#### 1. 컨테이너(Container) 관련

- **GET `/go/containers?all=true`**
  - 컨테이너 목록 조회 (`all=true` 이면 중지된 컨테이너 포함)
- **POST `/go/containers`**
  - 컨테이너 생성  
  - Body (`types.CreateContainerRequest`):

    ```json
    {
      "image": "nginx:latest",
      "name": "my-nginx",
      "cmd": [],
      "env": [],
      "platform": "linux/amd64"
    }
    ```

- **POST `/go/containers/{id}/start`**
- **POST `/go/containers/{id}/stop`**
- **DELETE `/go/containers/{id}`**
- **POST `/go/containers/prune`**
- **GET `/go/containers/{id}/logs`**
- **GET `/go/containers/{id}/stats`**
- **POST `/go/containers/{id}/exec`**

#### 2. 이미지(Image) 관련

- **GET `/go/images`**
- **POST `/go/images/build`**
  - Body (`types.BuildImageRequest`):

    ```json
    {
      "image_name": "myapp:latest",
      "dockerfile": "FROM nginx:alpine\n...",
      "context_path": ".",
      "platform": "linux/amd64"
    }
    ```

#### 3. 볼륨(Volume) 관련

- **GET `/go/volumes`**
- **GET `/go/volumes/{name}`**
- **POST `/go/volumes`**
- **DELETE `/go/volumes/{name}`**
- **POST `/go/volumes/prune`**
- **GET `/go/volumes/{name}/browse?path=/`**
  - 내부에서 `docker run --rm -v <volume>:/volume alpine ls -la ...` 실행 후 결과를 파싱하여 JSON으로 반환

#### 4. Compose / 파일 관련

- **POST `/go/files/compose`**
  - `docker-compose.yml` 등 Compose 파일 저장
- **POST `/go/files/nginx`**
  - `nginx.conf` 저장


