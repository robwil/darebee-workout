package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/vision/apiv1"
	"github.com/memcachier/mc"
	"github.com/qiangxue/fasthttp-routing"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
)

// TODO: eventually make this interactive with https://github.com/kataras/iris/#learn
// TODO: I can probably make a simple <Select> box of workouts, which map to their workout names

func detectText(imageURL string) (string, error) {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return "", err
	}
	image := vision.NewImageFromURI(imageURL)
	annotations, err := client.DetectDocumentText(ctx, image, nil)
	if err != nil {
		return "", err
	}
	return annotations.Text, nil
}

var exceptions = map[string]string{
	"alt-arm-leg-raises": "arm-leg-raises",
	"lunges-exercise":    "forward-lunges",
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

func getImageURL(workout string, day string) (string, error) {
	workout = strings.ToLower(workout)
	dayNum, err := strconv.Atoi(day)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://darebee.com/images/programs/%s/web/day%02d.jpg", workout, dayNum), nil
}

func getVideoURL(name string) string {
	return fmt.Sprintf("https://darebee.com/exercises/%s.html", name)
}

func getYoutubeEmbed(videoURL string) (string, error) {
	resp, err := http.Get(videoURL)
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

type exercise struct {
	name     string
	embedURL string
}

func getExercisesForImage(imageURL string) ([]exercise, error) {
	text, err := detectText(imageURL)
	if err != nil {
		return nil, err
	}
	var exercises []exercise
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		videoName := getVideoName(line)
		if videoName == "" {
			continue
		}
		URL := getVideoURL(videoName)
		embedURL, err := getYoutubeEmbed(URL)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise{name: line, embedURL: embedURL})
	}
	return exercises, nil
}

func printVideos(cn *mc.Client) func(*routing.Context) error {
	// TODO: handle err from fmt.Fprintf calls
	return func(ctx *routing.Context) error {
		log.Printf("GET %s", ctx.RequestURI())
		workout := ctx.Param("workout")
		day := ctx.Param("day")
		imageURL, err := getImageURL(workout, day)
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx, `<img src="%s" /><br/>`, imageURL)
		exercises, err := getExercisesForImage(imageURL)
		if err != nil {
			return err
		}
		for _, exercise := range exercises {
			if exercise.embedURL != "" {
				fmt.Fprintf(ctx, `
                    <h2>%s</h2>
                    <p>
                        <iframe width="845" height="480" src="//www.youtube.com/embed/%s?rel=0&showinfo=0" frameborder="0" allowfullscreen></iframe>
                    </p>`, exercise.name, exercise.embedURL)
			} else {
				fmt.Fprintf(ctx, `
                    <h2>%s</h2>
                    <p>Video not found</p>
                `, exercise.name)
			}
		}
		ctx.Response.Header.Set("Content-Type", "text/html")
		return nil
	}
}

func main() {
	// Setup memcached connection
	memcachedHost := os.Getenv("MEMCACHED_HOST")
	memcachedPort := os.Getenv("MEMCACHED_PORT")
	memcachedUser := os.Getenv("MEMCACHED_USER")
	memcachedPass := os.Getenv("MEMCACHED_PASS")
	mem := mc.NewMC(fmt.Sprintf("%s:%s", memcachedHost, memcachedPort), memcachedUser, memcachedPass)
	defer mem.Quit()

	// Setup http routes
	router := routing.New()
	router.Get("/<workout>/<day>", printVideos(mem))

	log.Printf("Listening on port 5000")
	if err := fasthttp.ListenAndServe("0.0.0.0:5000", router.HandleRequest); err != nil {
		log.Fatalf("error in ListenAndServe: %s", err)
	}
}
