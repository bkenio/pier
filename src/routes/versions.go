package routes

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bken-io/api/src/db"
	"github.com/bken-io/api/src/models"
	"github.com/bken-io/api/src/s3"
	jwt "github.com/form3tech-oss/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

func removeDuplicatesFromSlice(s []string) []string {
	m := make(map[string]bool)
	for _, item := range s {
		if _, ok := m[item]; ok {
		} else {
			m[item] = true
		}
	}
	var result []string
	for item := range m {
		result = append(result, item)
	}
	return result
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// fetches the master playlist from wasabi and returns preset names
func getMasterPlaylistPresets(id string) []string {
	// get the master playlist
	presets := []string{}
	s3Path := fmt.Sprintf("v/%s/hls/master.m3u8", id)

	object, err := s3.Wasabi().GetObject(
		context.Background(),
		"cdn.bken.io",
		s3Path,
		minio.GetObjectOptions{})
	if err != nil {
		fmt.Println(err)
		return nil
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(object)
	myFileContentAsString := buf.String()
	temp := strings.Split(myFileContentAsString, "\n")

	for i := 0; i < len(temp); i++ {
		e := temp[i]
		if strings.Contains(e, "#EXT-X-STREAM-INF:") {
			preset := strings.Split(e, "RESOLUTION=")[1]   // 1920x1080,URI...
			preset = strings.Split(preset, ",")[0]         // 1920x1080
			preset = strings.Split(preset, "x")[1]         // 1080
			preset = strings.Replace(preset, "\r", "", -1) // wtf is this. why does this string contain \r
			if preset != "" {
				presets = append(presets, preset)
			}
		}
	}

	return presets
}

func getNumberOfS3Objects(client *minio.Client, bucket string, prefix string) float32 {
	var numObjects float32

	opts := minio.ListObjectsOptions{
		Recursive: false,
		Prefix:    prefix,
	}
	for object := range s3.Doco().ListObjects(context.Background(), bucket, opts) {
		if object.Err != nil {
			fmt.Println(object.Err)
		}
		numObjects++
	}
	return numObjects
}

func calculatePercentCompleted(id string, version string) uint8 {
	totalSegments := getNumberOfS3Objects(s3.Doco(), "tidal", fmt.Sprintf("%s/segments/", id))
	transcodedSegments := getNumberOfS3Objects(s3.Doco(), "tidal", fmt.Sprintf("%s/versions/%s/segments/", id, version))
	percentCompleted := uint8((transcodedSegments / totalSegments) * 100)
	return percentCompleted
}

// GetVersions returns tidal information from s3
func GetVersions(c *fiber.Ctx) error {
	id := c.Params("id")

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	userID := claims["id"].(string)

	db := db.DBConn
	var video models.Video
	result := db.Where("id = ?", id).Find(&video)

	if result.Error != nil {
		return c.SendStatus(500)
	}

	if video.UserID != userID {
		return c.SendStatus(403)
	}

	versionsFromCDN := getMasterPlaylistPresets(id)
	// versionsFromTidal := getTidalVersions(id)

	// allVersions := append(versionsFromCDN, versionsFromTidal...)
	// uniqueVersions := removeDuplicatesFromSlice(allVersions)

	versions := []models.VideoVersion{}

	for i := 0; i < len(versionsFromCDN); i++ {
		version := versionsFromCDN[i]
		if contains(versionsFromCDN, version) {
			versions = append(versions, models.VideoVersion{
				PercentCompleted: 100,
				Name:             version,
				Status:           "completed",
			})
		} else {
			versions = append(versions, models.VideoVersion{
				PercentCompleted: 0,
				Name:             version,
				Status:           "processing",
			})
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		a, aErr := strconv.Atoi(versions[i].Name)
		b, bErr := strconv.Atoi(versions[j].Name)
		if aErr != nil || bErr != nil {
			fmt.Println(aErr, bErr)
			panic("failed to parse ints")
		}
		return a > b
	})
	return c.JSON(versions)
}
