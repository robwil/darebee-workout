package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestGetVideoName(t *testing.T) {
	t.Run("basic case", func(t *testing.T) {
		assert.Equal(t, "knee-strikes", getVideoName("20 knee strikes"))
		assert.Equal(t, "low-front-kicks", getVideoName("20 low front kicks"))
		assert.Equal(t, "overhead-punches", getVideoName("20 overhead punches"))
	})
	t.Run("handle extra space", func(t *testing.T) {
		assert.Equal(t, "knee-strikes", getVideoName("20  knee  strikes"))
	})
	t.Run("handle pluses", func(t *testing.T) {
		assert.Equal(t, "jab-jab-cross", getVideoName("20 jab + jab + cross"))
	})
	t.Run("things that should be ignored", func(t *testing.T) {
		assert.Equal(t, "", getVideoName("Foundation"))
		assert.Equal(t, "", getVideoName("Day 3 Fighter"))
		assert.Equal(t, "", getVideoName("Levell 3 sets"))
		assert.Equal(t, "", getVideoName("Level II 5 sets"))
		assert.Equal(t, "", getVideoName("Level III 7 sets"))
		assert.Equal(t, "", getVideoName("o darebee.com"))
		assert.Equal(t, "", getVideoName("2 minutes rest between sets"))
	})
	t.Run("case insensitivity", func(t *testing.T) {
		assert.Equal(t, "knee-strikes", getVideoName("20 Knee Strikes"))
		assert.Equal(t, "overhead-punches", getVideoName("20 OVerhead PunchES"))
		assert.Equal(t, "", getVideoName("2 minutes REST beTweeN sets"))
	})
	t.Run("single word exercises", func(t *testing.T) {
		assert.Equal(t, "bridges-exercise", getVideoName("10 bridges"))
		assert.Equal(t, "skiers-exercise", getVideoName("20 skiers"))
	})
	t.Run("exceptions", func(t *testing.T) {
		assert.Equal(t, "arm-leg-raises", getVideoName("10 alt arm / leg raises"))
		assert.Equal(t, "forward-lunges", getVideoName("20 lunges"))
	})
}

func TestGetImageURL(t *testing.T) {
	t.Run("basic case", func(t *testing.T) {
		imageURL, err := getImageURL("foundation", "23")
		assert.NilError(t, err)
		assert.Equal(t, "https://darebee.com/images/programs/foundation/web/day23.jpg", imageURL)
	})
	t.Run("pad zeroes", func(t *testing.T) {
		imageURL, err := getImageURL("foundation", "3")
		assert.NilError(t, err)
		assert.Equal(t, "https://darebee.com/images/programs/foundation/web/day03.jpg", imageURL)
	})
	t.Run("case insensitive", func(t *testing.T) {
		imageURL, err := getImageURL("Foundation", "3")
		assert.NilError(t, err)
		assert.Equal(t, "https://darebee.com/images/programs/foundation/web/day03.jpg", imageURL)
	})
	t.Run("number parsing", func(t *testing.T) {
		imageURL, err := getImageURL("foundation", "004")
		assert.NilError(t, err)
		assert.Equal(t, "https://darebee.com/images/programs/foundation/web/day04.jpg", imageURL)
	})
}

// TODO: refactor this function so the test doesn't make actual HTTP request
func TestGetYoutubeEmbed(t *testing.T) {
	embedURL, err := getYoutubeEmbed("https://darebee.com/exercises/burpees-with-push-up.html")
	assert.NilError(t, err)
	assert.Equal(t, "ZQzikdjmkKg", embedURL)
}
