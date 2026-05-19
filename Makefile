.PHONY: build test test-examples clean run-examples

# Сборка интерпретатора
build:
	go build -o yasny .

# Все тесты: golden + examples + удалённые формы
test:
	go test ./tests -v

# Прогнать все примеры через собранный интерпретатор
run-examples: build
	@for f in examples/*.ya; do \
		echo "=== $$f ==="; \
		./yasny "$$f"; \
	done

# Уборка
clean:
	rm -f yasny
