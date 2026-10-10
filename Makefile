outfile="mygofmt"

all:
	go build -buildvcs=false -o ${outfile} .

install:
	sudo /bin/cp ./${outfile} ~/bin/

install_cfg:
	sudo /bin/cp ./mygofmt.yaml.example ~/.config/mygofmt.yaml

clean:
	rm -f ${outfile}

