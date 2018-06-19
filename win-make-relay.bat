rem go build -ldflags -s -a -v -o build/bin/relay ./cmd/lrc/*
rem go install ./cmd/lrc/*
del lrc.exe
del relay.exe
cd cmd/lrc
go build -a
move lrc.exe ../../
cd ../../
rename lrc.exe relay.exe
