.PHONY: help run spike1 spike2 compare clean stats

help:
	@echo "GitHelper Corpus Validator - Make targets"
	@echo ""
	@echo "Setup:"
	@echo "  make setup          - Copy template and show next steps"
	@echo ""
	@echo "Running:"
	@echo "  make run            - Run with default settings"
	@echo "  make spike1         - Run Spike 1 (before implementation)"
	@echo "  make spike2         - Run Spike 2 (after implementation)"
	@echo "  make compare        - Run Spike 2 and compare with Spike 1"
	@echo ""
	@echo "Cache:"
	@echo "  make stats          - Show cache statistics"
	@echo "  make clean-cache    - Clear entire cache"
	@echo "  make clean-expired  - Clear expired cache entries"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean          - Clean generated reports"
	@echo ""

setup:
	@if [ ! -f repos.yaml ]; then \
		cp repos.yaml.template repos.yaml; \
		echo "✓ Created repos.yaml from template"; \
		echo ""; \
		echo "Next steps:"; \
		echo "  1. Edit repos.yaml and add your managed repos"; \
		echo "  2. Run 'make spike1' to create baseline"; \
	else \
		echo "⚠ repos.yaml already exists"; \
	fi

run:
	go run .

spike1:
	@echo "Running Spike 1 (baseline before implementation)..."
	go run . --repos repos.yaml --output spike1.json --html spike1.html
	@echo ""
	@echo "✓ Spike 1 complete!"
	@echo "  - Results: spike1.json"
	@echo "  - Report:  spike1.html"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Review spike1.html in browser"
	@echo "  2. Fix any clone failures"
	@echo "  3. Commit spike1.json: git add spike1.json && git commit -m 'Add corpus baseline'"
	@echo "  4. Implement classifier (Phases 1-4)"
	@echo "  5. Run 'make spike2' after implementation"

spike2:
	@echo "Running Spike 2 (validation after implementation)..."
	go run . --repos repos.yaml --output spike2.json --html spike2.html
	@echo ""
	@echo "✓ Spike 2 complete!"
	@echo "  - Results: spike2.json"
	@echo "  - Report:  spike2.html"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Review spike2.html in browser"
	@echo "  2. Check false positive rate < 5%"
	@echo "  3. Run 'make compare' to see changes from baseline"

compare:
	@echo "Running Spike 2 with comparison to Spike 1..."
	@if [ ! -f spike1.json ]; then \
		echo "⚠ Error: spike1.json not found. Run 'make spike1' first."; \
		exit 1; \
	fi
	go run . --repos repos.yaml --output spike2.json --html spike2.html --compare spike1.json
	@echo ""
	@echo "✓ Comparison complete!"
	@echo "  - Check console output for changes"
	@echo "  - Review spike2.html for details"

stats:
	go run . --cache-stats

clean-cache:
	go run . --clear-cache

clean-expired:
	go run . --clear-expired

clean:
	rm -f *.html
	@echo "✓ Cleaned generated reports"

# Development targets
test:
	go test -v .

build:
	go build -o corpus-validator .

install: build
	mv corpus-validator $(GOPATH)/bin/

.DEFAULT_GOAL := help
