package main

import (
    "github.com/valyala/fasthttp"
    "log"
    "github.com/qiangxue/fasthttp-routing"
    "io"
    "cloud.google.com/go/vision/apiv1"
    "golang.org/x/net/context"
    "strings"
    "fmt"
    "regexp"
    "strconv"
    "net/http"
    "io/ioutil"
)

// TODO: eventually make this interactive with https://github.com/kataras/iris/#learn

func detectText(w io.Writer, file string) (string, error) {
    ctx := context.Background()
    client, err := vision.NewImageAnnotatorClient(ctx)
    if err != nil {
        return "", err
    }
    image := vision.NewImageFromURI(file)
    annotations, err := client.DetectDocumentText(ctx, image, nil)
    if err != nil {
        return "", err
    }
    return annotations.Text, nil
}

var exceptions = map[string]string{
    "alt-arm-leg-raises": "arm-leg-raises",
    "lunges-exercise": "forward-lunges",
}

func getVideoName(line string) string {
    line = strings.ToLower(line)
    // extract names only when prefaced with exercise count
    r := regexp.MustCompile("^(\\d+)\\s+(.+)")
    matches := r.FindStringSubmatch(line)
    if len(matches) >= 3 {
        // handle "between sets" instruction; not an exercise so skip it
        if strings.Contains(line, "between") {
            return ""
        }
        // replace non-word chars with hyphen
        r = regexp.MustCompile("[^\\w]")
        str := r.ReplaceAllString(matches[2], "-")
        // convert any multi hyphen to hyphen (making less sensitive to Google Vision mistakes)
        r = regexp.MustCompile("-+")
        str = r.ReplaceAllString(str, "-")
        // for single word exercises, they append "-exercise" to it
        if !strings.Contains(str, "-") {
            str = str + "-exercise"
        }
        // check for any exceptional cases
        if exceptions[str] != "" {
            return exceptions[str]
        }
        return str
    }
    return ""
}

// TODO: I can probably make a simple <Select> box of workouts, which map to their workout names

func getImageUrl(workout string, day string) (string, error) {
    workout = strings.ToLower(workout)
    dayNum, err := strconv.Atoi(day)
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("https://darebee.com/images/programs/%s/web/day%02d.jpg", workout, dayNum), nil
}

func getVideoUrl(name string) string {
    return fmt.Sprintf("https://darebee.com/exercises/%s.html", name)
}

func getYoutubeEmbed(videoUrl string) (string, error) {
    resp, err := http.Get(videoUrl)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    r := regexp.MustCompile("youtube\\.com/embed/([^?]+)\\?")
    matches := r.FindStringSubmatch(string(body))
    if len(matches) >= 2 {
        return matches[1], nil
    }
    return "", nil
}

func PrintVideos(ctx *routing.Context) error {
    log.Printf("GET %s", ctx.RequestURI())
    workout := ctx.Param("workout")
    day := ctx.Param("day")
    imageUrl, err := getImageUrl(workout, day)
    fmt.Fprintf(ctx, `<img src="%s" /><br/>`, imageUrl)
    if err != nil {
        return err
    }
    text, err :=  detectText(ctx, imageUrl)
    if err != nil {
        return err
    }
    lines := strings.Split(text, "\n")
    for _, line := range lines {
        videoName := getVideoName(line)
        if videoName == "" {
            continue
        }
        url := getVideoUrl(videoName)
        embedUrl, err := getYoutubeEmbed(url)
        if err != nil {
            return err
        }
        if embedUrl != "" {
            fmt.Fprintf(ctx,`
                <h2>%s</h2>
                <p>
                    <a href="%s" target="_blank">%s</a> <br/>
                    <iframe width="845" height="480" src="//www.youtube.com/embed/%s?rel=0&showinfo=0" frameborder="0" allowfullscreen></iframe>
                </p>`, line, url, url, embedUrl)
        } else {
            fmt.Fprintf(ctx, `
                <h2>%s</h2>
                <p>Video not found</p>
            `, line)
        }
    }
    ctx.Response.Header.Set("Content-Type", "text/html")
    return nil
}

func main() {
    router := routing.New()
    router.Get("/<workout>/<day>", PrintVideos)
    log.Printf("Listening on port 5000")
    if err := fasthttp.ListenAndServe("0.0.0.0:5000", router.HandleRequest); err != nil {
        log.Fatalf("error in ListenAndServe: %s", err)
    }
}
