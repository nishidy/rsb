rsb: *go
	go build -o $@ *go

.PHONY: install
install:
	cp rsb /usr/bin/

.PHONY: clean
clean:
	rm rsb

