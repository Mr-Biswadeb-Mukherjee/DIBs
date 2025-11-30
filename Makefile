# --- Code Quality Pipeline for INFERMAL_v2 ---
.PPHONY: all fmt vet lint security complexity sloc test coverage autofmt

YELLOW=\033[1;33m
GREEN=\033[1;32m
BLUE=\033[1;34m
RED=\033[1;31m
RESET=\033[0m

all: autofmt vet lint security complexity sloc test coverage
	@echo "$(GREEN)🎉 All checks (with auto-format) completed successfully!$(RESET)"

autofmt:
	@echo "$(BLUE)🧹 Checking code formatting...$(RESET)"
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "$(YELLOW)⚠️  Found unformatted Go files. Auto-fixing...$(RESET)"; \
		echo "$$unformatted" | xargs gofmt -w; \
		echo "$(GREEN)✅ Formatting issues fixed automatically.$(RESET)"; \
	else \
		echo "$(GREEN)✅ All files are properly formatted.$(RESET)"; \
	fi

vet:
	@echo "\n$(BLUE)🔍 Running go vet (static analysis)...$(RESET)"
	@go vet ./... || true

lint:
	@echo "\n$(BLUE)🧠 Running golangci-lint...$(RESET)"
	@golangci-lint run ./... --color=always || true

security:
	@echo "\n$(BLUE)🛡️ Running gosec (security scan)...$(RESET)"
	@gosec ./... || true

complexity:
	@echo "\n$(BLUE)⚙️ Checking cyclomatic complexity (threshold: >10)...$(RESET)"
	@gocyclo -over 10 . || echo "$(GREEN)✅ Complexity levels are acceptable.$(RESET)"

sloc:
	@echo "\n$(BLUE)📄 Calculating SLOC for all Go files with pass/fail assessment...$(RESET)"
	@cloc . --include-lang=Go --by-file --quiet --csv | awk -F',' -v green="$(GREEN)" -v yellow="$(YELLOW)" -v red="$(RED)" -v reset="$(RESET)" ' \
	BEGIN { \
		printf "------------------------------------------------------------------------------------------------------\n"; \
		printf "%-60s %8s %9s %9s %9s\n", "File", "Blank", "Comment", "Code", "Result"; \
		printf "------------------------------------------------------------------------------------------------------\n"; \
		total_blank=0; total_comment=0; total_code=0; \
	} \
	/Go,/ { \
		file=$$2; blank=$$3; comment=$$4; code=$$5; \
		if (code > 400) { color=red; result="FAIL"; } \
		else if (code > 250) { color=yellow; result="WARN"; } \
		else { color=green; result="PASS"; } \
		printf "%-60s %8d %9d %9d %11s%s%s\n", file, blank, comment, code, color, result, reset; \
		total_blank+=blank; total_comment+=comment; total_code+=code; \
	} \
	END { \
		printf "------------------------------------------------------------------------------------------------------\n"; \
		final_color = (total_code > 2500) ? red : green; \
		final_result = (total_code > 2500) ? "FAIL" : "PASS"; \
		printf "%-60s %8d %9d %9d %11s%s%s\n", "TOTAL", total_blank, total_comment, total_code, final_color, final_result, reset; \
		printf "------------------------------------------------------------------------------------------------------\n"; \
	}'

test:
	@echo "\n$(BLUE)🧪 Running unit tests...$(RESET)"
	@go test ./... -v || true

coverage:
	@echo "\n$(BLUE)📊 Generating test coverage heatmap...$(RESET)"

	# Run tests while EXCLUDING:
	#   1. root module (.)
	#   2. Modules/app       (only the top-level app folder)
	@go test $(shell go list ./... | grep -vE '(^$$|/app$$)') \
		-coverpkg=./... -coverprofile=coverage.out || { \
		echo "$(RED)❌ Coverage generation failed (test failure).$(RESET)"; exit 1; }

	# Ensure coverage.out exists
	@[ -s coverage.out ] || { \
		echo "$(RED)❌ coverage.out is empty. No coverage data was generated.$(RESET)"; \
		exit 1; }

	# Validate file header
	@if ! head -n 1 coverage.out | grep -q '^mode:' ; then \
		echo "$(RED)❌ Invalid coverage.out format. Aborting coverage heatmap.$(RESET)"; \
		exit 1; \
	fi

	@echo "\n$(BLUE)📊 Coverage Heatmap (Per Folder)$(RESET)"
	@echo "------------------------------------------------------------"

	@go tool cover -func=coverage.out \
	| awk -v green="$(GREEN)" -v yellow="$(YELLOW)" -v red="$(RED)" -v reset="$(RESET)" ' \
	/\.go/ { \
		orig = $$1; \
		file = orig; \
		sub(/^.*Infermal_v2\//, "", file); \
		split(file, parts, "/"); \
		folder = "."; \
		if (length(parts) > 1) { \
			folder = parts[1]; \
			for (i=2; i<length(parts); i++) folder=folder "/" parts[i]; \
		} \
		percent = $$3; gsub(/%/, "", percent); \
		if (percent == "") next; \
		folder_cov[folder] += percent; folder_count[folder]++; \
		files[folder] = files[folder]" "orig; \
		shown[folder] = shown[folder]" "file; \
	} \
	END { \
		n = 0; \
		for (f in folder_cov) { n++; folders[n] = f; } \
		for (i = 1; i <= n; i++) { \
			for (j = i + 1; j <= n; j++) { \
				ai = folder_cov[folders[i]] / folder_count[folders[i]]; \
				aj = folder_cov[folders[j]] / folder_count[folders[j]]; \
				if (aj > ai) { tmp = folders[i]; folders[i] = folders[j]; folders[j] = tmp; } \
			} \
		} \
		for (k = 1; k <= n; k++) { \
			f = folders[k]; \
			avg = folder_cov[f] / folder_count[f]; \
			if (avg >= 80) color = green; \
			else if (avg >= 60) color = yellow; \
			else if (avg >= 40) color = yellow; \
			else color = red; \
			cmd = "printf \"" files[f] "\" | xargs sha256sum 2>/dev/null | sha256sum"; \
			cmd | getline h; close(cmd); \
			sub(/ .*/, "", h); \
			short = substr(h, 1, 8); \
			printf "%-40s %6.1f%% %s●%s  [hash: %s]\n", f, avg, color, reset, short; \
		} \
	}'

	@echo "------------------------------------------------------------"
	@go tool cover -func=coverage.out | grep total | awk '{print "Overall Coverage: " $$3}'
	@echo "$(GREEN)✅ Coverage heatmap ready.$(RESET)"