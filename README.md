Golang binding for ffmpeg

## Linking ffmpeg libraries

### Golang

Before building any Golang examples add following environment variables:

```bash
export FFMPEG_ROOT=$HOME/ffmpeg
export CGO_LDFLAGS="-L$FFMPEG_ROOT/lib/ -lavcodec -lavformat -lavutil -lswscale -lswresample -lavdevice -lavfilter"
export CGO_CFLAGS="-I$FFMPEG_ROOT/include"
export LD_LIBRARY_PATH=$HOME/ffmpeg/lib
export PKG_CONFIG_PATH=$LD_LIBRARY_PATH/pkgconfig/
```

And then build C example with following command:
```bash
go build example.go
```

### C

Before building any C examples add following environment variables:
```bash
export PATH=$FFMPEG_ROOT:$PATH
export LDFLAGS="-L$FFMPEG_ROOT/lib/ -lavcodec -lavformat -lavutil -lswscale -lswresample -lavdevice -lavfilter"
export CFLAGS="-I$FFMPEG_ROOT/include"
```

And then build C example with following command:
```bash
gcc example.c -o example_c -lavcodec -lavformat -lavutil -lswscale -lswresample -lavdevice -lavfilter
```
