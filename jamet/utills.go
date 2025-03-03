package jamet

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/google/uuid"
)

// get UUID
func UUID() string {
	return uuid.New().String()
}

// return JSON status true
func PrintJSON(c *gin.Context, response any) {
	c.Render(http.StatusOK, render.JSON{Data: response})
}

// return JSON status false
func EPrintJSON(c *gin.Context, response any) {
	c.Render(http.StatusBadRequest, render.JSON{Data: response})
}

func ArrayKey(ar map[string]interface{}) []string {

	keys := make([]string, 0, len(ar))
	for k := range ar {
		keys = append(keys, k)
	}

	return keys
}

func Contains(s []string, str string) (bool, string) {
	for i, v := range s {
		if strings.Contains(v, str) {
			index := fmt.Sprintf("%d", i)
			return true, index
		}
	}
	return false, ""
}

func Validation(request map[string]interface{}, format map[string]map[string]string) string {

	var errMessage []string
	message := format["message"]
	alias := format["alias"]
	for key, value := range format["field"] {

		var contain bool
		var index string
		cond := strings.Split(value, "|")
		formData := request[key].(string)

		contain, _ = Contains(cond, "required")
		if contain && formData == "" {

			if len(message) > 0 && message[key] != "" {
				errMessage = append(errMessage, message[key])
			} else {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("%s perlu diisi", alias[key]))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("%s perlu diisi", key))
				}
			}

			continue
		}

		//min
		contain, index = Contains(cond, "min")
		if contain {

			i, err := strconv.Atoi(index)
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			arr := strings.Split(cond[i], ":")
			min, err := strconv.Atoi(arr[1])
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			if len(formData) < min {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s kurang dari %d", alias[key], min))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s kurang dari %d", key, min))
				}
				continue
			}
		}

		//max
		contain, index = Contains(cond, "max")
		if contain {

			i, err := strconv.Atoi(index)
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			arr := strings.Split(cond[i], ":")
			max, err := strconv.Atoi(arr[1])
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			if len(formData) > max {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s lebih dari %d", alias[key], max))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s lebih dari %d", key, max))
				}
				continue
			}
		}
	}

	if len(errMessage) > 0 {
		return strings.Join(errMessage, " | ")
	} else {
		return ""
	}
}

func Md5(data []byte) string {

	return fmt.Sprintf("%x", md5.Sum(data))
}

func Converter(req any) map[string]interface{} {

	convert := fmt.Sprintf(`[%s]`, req)
	var jsonBlob = []byte(convert)
	var objmap []map[string]interface{}
	if err := json.Unmarshal(jsonBlob, &objmap); err != nil {
		log.Fatal(err)
	}

	return objmap[0]
}

func Unrupiah(nilai string) int {
	rupiah := strings.ReplaceAll(nilai, ",", "")
	result, err := strconv.Atoi(strings.ReplaceAll(rupiah, ".", ""))
	if err != nil {
		log.Fatalln(err)
	}

	return result
}
