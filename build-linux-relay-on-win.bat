SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
cd cmd/lrc
go build -a
move lrc ../../
cd ../../
rename lrc relay