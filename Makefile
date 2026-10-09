outfile="mygofmt"

all:
	go build -buildvcs=false -o ${outfile} .

install:
	sudo /bin/cp ./${outfile} ~/bin/

clean:
	rm -f ${outfile}

