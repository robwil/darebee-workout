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
	"github.com/bmizerany/mc"
	"github.com/qiangxue/fasthttp-routing"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
)

// TODO: eventually make this interactive with https://github.com/kataras/iris/#learn
// TODO: I can probably make a simple <Select> box of workouts, which map to their workout names

func detectText(imageUrl string) (string, error) {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return "", err
	}
	image := vision.NewImageFromURI(imageUrl)
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

type Exercise struct {
	name     string
	embedUrl string
}

func GetExercisesForImage(imageUrl string) ([]Exercise, error) {
	text, err := detectText(imageUrl)
	if err != nil {
		return nil, err
	}
	var exercises []Exercise
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		videoName := getVideoName(line)
		if videoName == "" {
			continue
		}
		url := getVideoUrl(videoName)
		embedUrl, err := getYoutubeEmbed(url)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, Exercise{name: line, embedUrl: embedUrl})
	}
	return exercises, nil
}

func PrintVideos(cn *mc.Conn) func(*routing.Context) error {
	// TODO: handle err from fmt.Fprintf calls
	return func(ctx *routing.Context) error {
		log.Printf("GET %s", ctx.RequestURI())
		workout := ctx.Param("workout")
		day := ctx.Param("day")
		imageUrl, err := getImageUrl(workout, day)
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx, `<img src="%s" /><br/>`, imageUrl)
		exercises, err := GetExercisesForImage(imageUrl)
		if err != nil {
			return err
		}
		for _, exercise := range exercises {
			if exercise.embedUrl != "" {
				fmt.Fprintf(ctx, `
                    <h2>%s</h2>
                    <p>
                        <iframe width="845" height="480" src="//www.youtube.com/embed/%s?rel=0&showinfo=0" frameborder="0" allowfullscreen></iframe>
                    </p>`, exercise.name, exercise.embedUrl)
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
	cn, err := mc.Dial("tcp", fmt.Sprintf("%s:%s", memcachedHost, memcachedPort))
	if err != nil {
		log.Fatalf("could not connect to memcached %s:%s : %v", memcachedHost, memcachedPort, err)
	}
	if memcachedUser != "" {
		if err = cn.Auth(memcachedUser, memcachedPass); err != nil {
			log.Fatalf("could not auth to memcached with user %s: %v", memcachedUser, err)
		}
	}

	// Setup http routes
	router := routing.New()
	router.Get("/<workout>/<day>", PrintVideos(cn))

	log.Printf("Listening on port 5000")
	if err := fasthttp.ListenAndServe("0.0.0.0:5000", router.HandleRequest); err != nil {
		log.Fatalf("error in ListenAndServe: %s", err)
	}
}
