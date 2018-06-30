# darebee-workout

## Background

Darebee.com offers many free workouts that can be done anywhere, without equipment. I have been enjoying these workouts,
but one frustration is that for a workout noob like me, the workout graphics don't always make it clear what to do for
any given exercise. The site addresses this by having a separate Video Library of most exercises, but there is no linking
between the workout graphic and the video for the exercises.

In order to address this frustration, I built a quick tool that will perform the following actions:

1) Given a workout id + day, via URL, fetch the image for that day's workout.
2) Use the Google Vision API to detect text in the workout image.
3) Use some regex to convert the detected exercise names into URLs where the videos are hosted.
4) Fetch those video pages, scraping for Youtube embed.
5) Outputting the workout graphic, followed by any found Youtube embeds from steps 3-4.

## Requirements

You will need a Google Cloud account. The application expects the JSON for a valid Service Account to be dropped in the root
of this directory, with the filename `.gcloud_darebee.json`

## Deployment

```
$ go test
$ docker build -t darebee .
$ docker tag darebee gcr.io/darebee-208813/darebee-workout:1.0
$ docker push gcr.io/darebee-208813/darebee-workout:1.0
```

Then refresh GCE instance from Google cloud console.