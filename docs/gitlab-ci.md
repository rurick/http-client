# GitLab CI/CD

Настройка непрерывной интеграции и развертывания для HTTP клиента в GitLab.

## Базовая конфигурация

### .gitlab-ci.yml

```yaml
# GitLab CI конфигурация для HTTP клиента
stages:
  - test
  - quality
  - security
  - build
  - deploy

variables:
  GO_VERSION: "1.23"
  GOLANGCI_LINT_VERSION: "v1.55"

# Шаблон для Go окружения
.go-template: &go-template
  image: golang:${GO_VERSION}
  before_script:
    - mkdir -p $GOPATH/src/$(dirname $REPO_NAME)
    - ln -svf $CI_PROJECT_DIR $GOPATH/src/$REPO_NAME
    - cd $GOPATH/src/$REPO_NAME
    - go mod download

# Unit тесты
test:unit:
  <<: *go-template
  stage: test
  script:
    - go test -v -race -coverprofile=coverage.out ./...
    - go tool cover -html=coverage.out -o coverage.html
  coverage: '/coverage: \d+\.\d+% of statements/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml
    paths:
      - coverage.html
      - coverage.out
    expire_in: 30 days
  only:
    - merge_requests
    - main
    - develop

# Интеграционные тесты
test:integration:
  <<: *go-template
  stage: test
  services:
    - name: httpbin/httpbin
      alias: httpbin
  variables:
    HTTPBIN_URL: "http://httpbin:80"
  script:
    - go test -v -tags=integration ./...
  only:
    - merge_requests
    - main
  allow_failure: true

# Benchmark тесты
test:benchmark:
  <<: *go-template
  stage: test
  script:
    - go test -bench=. -benchmem ./...
  artifacts:
    paths:
      - benchmark.out
    expire_in: 7 days
  only:
    - main
  allow_failure: true

# Качество кода
quality:lint:
  image: golangci/golangci-lint:${GOLANGCI_LINT_VERSION}
  stage: quality
  script:
    - golangci-lint run --out-format=junit-xml:report.xml --out-format=colored-line-number
  artifacts:
    reports:
      junit: report.xml
    paths:
      - report.xml
    expire_in: 7 days
  only:
    - merge_requests
    - main

# Проверка форматирования
quality:format:
  <<: *go-template
  stage: quality
  script:
    - test -z "$(gofmt -l .)"
    - go vet ./...
  only:
    - merge_requests
    - main

# Проверка зависимостей
quality:deps:
  <<: *go-template
  stage: quality
  script:
    - go mod tidy
    - go mod verify
    - test -z "$(git diff --name-only)"
  only:
    - merge_requests
    - main

# Анализ безопасности
security:gosec:
  image: securecodewarrior/gosec:latest
  stage: security
  script:
    - gosec -fmt=junit-xml -out=gosec-report.xml ./...
  artifacts:
    reports:
      junit: gosec-report.xml
    paths:
      - gosec-report.xml
    expire_in: 7 days
  only:
    - merge_requests
    - main
  allow_failure: true

# Сканирование зависимостей
security:deps:
  image: golang:${GO_VERSION}
  stage: security
  script:
    - go install golang.org/x/vuln/cmd/govulncheck@latest
    - govulncheck ./...
  only:
    - merge_requests
    - main
  allow_failure: true

# Сборка библиотеки
build:library:
  <<: *go-template
  stage: build
  script:
    - go build ./...
    - go build -o http-client-examples ./examples/...
  artifacts:
    paths:
      - http-client-examples
    expire_in: 1 day
  only:
    - main
    - tags

# Создание документации
build:docs:
  image: node:18-alpine
  stage: build
  before_script:
    - npm install -g @gitiles/markdown-to-html
  script:
    - mkdir -p public/docs
    - for file in docs/*.md; do
        filename=$(basename "$file" .md);
        markdown-to-html "$file" > "public/docs/$filename.html";
      done
    - cp README.md public/
  artifacts:
    paths:
      - public
    expire_in: 30 days
  only:
    - main

# Развертывание документации
deploy:pages:
  stage: deploy
  dependencies:
    - build:docs
  script:
    - echo "Развертывание GitLab Pages"
  artifacts:
    paths:
      - public
  only:
    - main

# Создание релиза
deploy:release:
  <<: *go-template
  stage: deploy
  before_script:
    - apt-get update && apt-get install -y git
  script:
    - |
      if [ -n "$CI_COMMIT_TAG" ]; then
        echo "Создание релиза $CI_COMMIT_TAG"
        # Здесь можно добавить команды для создания релиза
        # Например, загрузка артефактов в package registry
      fi
  only:
    - tags
  when: manual
```

## Дополнительные конфигурации

### .golangci.yml

```yaml
# golangci-lint конфигурация
run:
  timeout: 5m
  tests: true
  skip-dirs:
    - vendor
  skip-files:
    - ".*_test.go"

linters-settings:
  gofmt:
    simplify: true
  govet:
    check-shadowing: true
  gocyclo:
    min-complexity: 15
  dupl:
    threshold: 100
  goconst:
    min-len: 3
    min-occurrences: 3
  misspell:
    locale: US
  lll:
    line-length: 120
  gocritic:
    enabled-tags:
      - performance
      - style
      - experimental
    disabled-checks:
      - wrapperFunc

linters:
  enable:
    - gofmt
    - golint
    - govet
    - gocyclo
    - dupl
    - goconst
    - misspell
    - lll
    - gocritic
    - gosec
    - ineffassign
    - unconvert
    - goimports
  disable:
    - errcheck # Отключаем пока есть много игнорируемых ошибок

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gocyclo
        - dupl
        - gosec
```

### Переменные окружения

В настройках проекта GitLab необходимо добавить переменные:

```bash
# Secrets для тестирования внешних API (если нужно)
TEST_API_TOKEN=your-test-api-token

# Настройки для развертывания
DEPLOY_TOKEN=your-deploy-token

# Настройки для уведомлений
SLACK_WEBHOOK_URL=your-slack-webhook
```

## Шаблоны для разных окружений

### Разработка (Development)

```yaml
# Дополнительные правила для ветки develop
test:dev:
  <<: *go-template
  stage: test
  script:
    - go test -v -short ./...
  only:
    - develop
  except:
    - schedules

# Деплой в dev окружение
deploy:dev:
  stage: deploy
  script:
    - echo "Развертывание в dev окружение"
    # Команды для развертывания в dev
  environment:
    name: development
    url: https://dev-docs.example.com
  only:
    - develop
```

### Staging

```yaml
# Staging тесты
test:staging:
  <<: *go-template
  stage: test
  script:
    - go test -v -tags=staging ./...
  environment:
    name: staging
  only:
    - main
  when: manual

# Деплой в staging
deploy:staging:
  stage: deploy
  script:
    - echo "Развертывание в staging"
    # Команды для развертывания в staging
  environment:
    name: staging
    url: https://staging-docs.example.com
  only:
    - main
  when: manual
```

### Production

```yaml
# Production деплой
deploy:production:
  stage: deploy
  script:
    - echo "Развертывание в production"
    # Команды для production развертывания
  environment:
    name: production
    url: https://docs.example.com
  only:
    - tags
  when: manual
  allow_failure: false
```

## Кэширование

### Кэш для Go модулей

```yaml
# Добавляем в .gitlab-ci.yml
cache:
  key: "${CI_JOB_NAME}"
  paths:
    - .cache/go-build/
    - .cache/go-mod/

variables:
  GOCACHE: $CI_PROJECT_DIR/.cache/go-build
  GOMODCACHE: $CI_PROJECT_DIR/.cache/go-mod

before_script:
  - mkdir -p .cache/go-build .cache/go-mod
```

## Parallel matrix builds

### Тестирование на разных версиях Go

```yaml
test:matrix:
  <<: *go-template
  stage: test
  parallel:
    matrix:
      - GO_VERSION: ["1.21", "1.22", "1.23"]
  image: golang:${GO_VERSION}
  script:
    - go version
    - go test -v ./...
  only:
    - merge_requests
    - main
```

## Уведомления

### Slack интеграция

```yaml
# Добавляем в конец .gitlab-ci.yml
notify:success:
  stage: .post
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
  script:
    - |
      curl -X POST -H 'Content-type: application/json' \
        --data '{"text":"✅ Build succeeded for '"$CI_PROJECT_NAME"' on '"$CI_COMMIT_REF_NAME"'"}' \
        $SLACK_WEBHOOK_URL
  only:
    - main
  when: on_success

notify:failure:
  stage: .post
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
  script:
    - |
      curl -X POST -H 'Content-type: application/json' \
        --data '{"text":"❌ Build failed for '"$CI_PROJECT_NAME"' on '"$CI_COMMIT_REF_NAME"'"}' \
        $SLACK_WEBHOOK_URL
  only:
    - main
    - merge_requests
  when: on_failure
```

## Merge Request шаблоны

### .gitlab/merge_request_templates/default.md

```markdown
## Описание изменений

Кратко опишите что изменилось в этом MR.

## Тип изменений

- [ ] Bug fix (исправление ошибки)
- [ ] New feature (новая функциональность)
- [ ] Breaking change (изменения, нарушающие обратную совместимость)
- [ ] Documentation update (обновление документации)
- [ ] Code refactoring (рефакторинг без изменения функциональности)

## Чеклист

- [ ] Код следует стандартам проекта
- [ ] Добавлены/обновлены тесты
- [ ] Все тесты проходят
- [ ] Обновлена документация (если необходимо)
- [ ] Добавлены changelog записи (если необходимо)
- [ ] Code review пройден

## Тестирование

Опишите как тестировались изменения:

- [ ] Unit тесты
- [ ] Integration тесты
- [ ] Manual тестирование

## Связанные issues

Closes #XXX
Relates to #XXX
```

## Issue шаблоны

### .gitlab/issue_templates/bug.md

```markdown
## Описание проблемы

Четкое описание того, что не работает.

## Шаги для воспроизведения

1. 
2. 
3. 

## Ожидаемое поведение

Что должно было произойти.

## Актуальное поведение

Что происходит на самом деле.

## Окружение

- Go версия: 
- OS: 
- HTTP клиент версия: 

## Дополнительная информация

Логи, скриншоты, примеры кода.
```

### .gitlab/issue_templates/feature.md

```markdown
## Описание функциональности

Опишите желаемую функциональность.

## Мотивация

Почему эта функциональность нужна?

## Подробное описание

Детальное описание реализации.

## Альтернативы

Рассматривались ли альтернативные решения?

## Дополнительная информация

Примеры использования, диаграммы, etc.
```

## Мониторинг CI/CD

### Metrics и алерты

```yaml
# Добавляем job для отправки метрик
metrics:ci:
  stage: .post
  image: alpine:latest
  script:
    - |
      # Отправка метрик в систему мониторинга
      curl -X POST "https://metrics.example.com/ci" \
        -d "project=$CI_PROJECT_NAME" \
        -d "branch=$CI_COMMIT_REF_NAME" \
        -d "duration=$CI_JOB_STARTED_AT" \
        -d "status=$CI_JOB_STATUS"
  when: always
  only:
    - main
```

## Лучшие практики

### 1. Быстрые feedback циклы
- Unit тесты должны выполняться быстро (< 2 минут)
- Используйте параллельное выполнение
- Кэшируйте зависимости

### 2. Стабильность pipeline
- Не делайте тесты зависимыми от внешних сервисов
- Используйте фиксированные версии образов
- Добавляйте retry для нестабильных шагов

### 3. Безопасность
- Никогда не логируйте секреты
- Используйте protected variables для чувствительных данных
- Регулярно сканируйте зависимости

### 4. Качество кода
- Настройте качественные проверки (linting, formatting)
- Требуйте минимальное покрытие тестами
- Используйте статический анализ

## См. также

- [Конфигурация](configuration.md) - Настройка клиента для разных окружений
- [Тестирование](testing.md) - Стратегии тестирования
- [GitLab CI/CD Documentation](https://docs.gitlab.com/ee/ci/) - Официальная документация GitLab CI/CD