FROM golang:1.18 as builder
COPY . /build
WORKDIR /build/service
ENV ALLURE_RESULTS_PATH=/build
RUN go test -gcflags=-l --coverprofile=coverage.out; exit 0
RUN mkdir -p coverage-report
RUN go tool cover -html=coverage.out -o coverage-report/report.html


FROM scratch as exporter
COPY --from=builder /build/allure-results /allure-results
COPY --from=builder /build/service/coverage-report /coverage-report
