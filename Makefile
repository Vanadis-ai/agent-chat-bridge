.PHONY: build run stop start restart test test-race test-all lint clean logs

PIDFILE := /tmp/telebridge.pid
LOGFILE := /tmp/telebridge.log
CONFIG := configs/config.yaml

build:
	go build -o telebridge ./cmd/telebridge

stop:
	@if [ -f $(PIDFILE) ]; then \
		pid=$$(cat $(PIDFILE)); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "Stopping telebridge (PID $$pid)"; \
			kill $$pid; \
			for i in 1 2 3 4 5; do \
				kill -0 $$pid 2>/dev/null || break; \
				sleep 1; \
			done; \
			if kill -0 $$pid 2>/dev/null; then \
				echo "Force killing $$pid"; \
				kill -9 $$pid; \
				sleep 1; \
			fi; \
		fi; \
		rm -f $(PIDFILE); \
	fi
	@echo "Telebridge stopped"

start: build stop
	@nohup sh -c 'unset CLAUDECODE && export $$(cat .env | xargs) && exec ./telebridge --config $(CONFIG) --pidfile $(PIDFILE) --debug' > $(LOGFILE) 2>&1 &
	@sleep 2
	@if [ -f $(PIDFILE) ] && kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		echo "Telebridge started (PID $$(cat $(PIDFILE)))"; \
	else \
		echo "ERROR: telebridge failed to start. Check $(LOGFILE)"; \
		exit 1; \
	fi

restart: start

run: build
	export $$(cat .env | xargs) && exec ./telebridge --config $(CONFIG)

logs:
	@tail -f $(LOGFILE)

test:
	go test ./...

test-race:
	go test -race ./...

test-all: test test-race

lint:
	go vet ./...

clean:
	rm -f telebridge
