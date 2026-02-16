.PHONY: build flash monitor clean

TARGET  ?= pico
OUT_DIR  = build
OUT_FILE = $(OUT_DIR)/f-tap.uf2

build:
	@mkdir -p $(OUT_DIR)
	tinygo build -target=$(TARGET) -size short -o $(OUT_FILE) .
	@echo "Built: $(OUT_FILE)"

flash:
	tinygo flash -target=$(TARGET) -size short .

monitor:
	tinygo monitor -baudrate=115200

clean:
	rm -rf $(OUT_DIR)
