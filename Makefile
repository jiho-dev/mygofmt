all:
	go build -buildvcs=false -o mygofmt .

install:
	sudo /bin/cp ./mygofmt ~/bin/

