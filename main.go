package main

import (
    "github.com/valyala/fasthttp"
    "log"
    "github.com/qiangxue/fasthttp-routing"
    "fmt"
)

// TODO: eventually make this interactive with https://github.com/kataras/iris/#learn

func Hello(ctx *routing.Context) error {
    log.Printf("GET %s", ctx.RequestURI())
    fmt.Fprintf(ctx, "Hello world")
    return nil
}

func main() {
    router := routing.New()
    router.Get("/", Hello)
    if err := fasthttp.ListenAndServe("127.0.0.1:5000", router.HandleRequest); err != nil {
        log.Fatalf("error in ListenAndServe: %s", err)
    }
}
