package jamethelper

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Jamet struct {
	Config map[string]*gorm.DB
	Redis  FormatRedis
}

type FormatRedis struct {
	Host, Port, Password string
	Database             int
	On                   bool
}

func NewJamet(param Jamet) *Jamet {
	return &Jamet{
		Config: param.Config,
		Redis:  param.Redis,
	}
}

func (met Jamet) GetData(table string, connection string) *gorm.DB {

	return met.Config[connection].Table(table)
}

func (met Jamet) Connection(conn string) *gorm.DB {
	db := met.Config[conn]

	return db.Begin()
}

func (met Jamet) SinchronizeID(db *gorm.DB, id string, char string, format int32) string {
	defer ErrorLog()

	var getMstRunNum map[string]interface{}
	db.Table("mst_run_nums").Where(map[string]interface{}{"val_id": id, "val_char": char}).Find(&getMstRunNum)

	var value string
	if len(getMstRunNum) != 0 {

		num, err := strconv.Atoi(getMstRunNum["val_value"].(string))
		if err != nil {
			panic(err)
		}

		num = num + 1
		db.Table("mst_run_nums").Where(map[string]interface{}{"val_id": id, "val_char": char}).Updates(map[string]interface{}{"val_value": num})

		value = fmt.Sprintf("%0*d", format, num)
	} else {
		value = fmt.Sprintf("%0*d", format, 1)
		InsertData(db, "mst_run_nums", map[string]interface{}{
			"id":        UUID(),
			"val_value": 1,
			"val_id":    id,
			"val_char":  char,
		})
	}

	return fmt.Sprintf("%s%s%s", id, char, value)
}

func (met Jamet) ReadCache(previx string) (bool, map[string]interface{}) {

	ctx := context.Background()

	format := met.Redis

	if format.On {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		val, err := client.Get(ctx, previx).Result()
		if err != nil {
			return false, map[string]interface{}{}
		}

		convert := Converter(val)

		return true, convert

	} else {
		return false, map[string]interface{}{}
	}
}

func (met Jamet) WriteCache(previx string, data any) {

	ctx := context.Background()

	format := met.Redis

	if format.On {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		jsonStr, err := json.Marshal(data)
		if err != nil {
			fmt.Println(err)
		}

		res, err := client.Set(ctx, previx, jsonStr, 0).Result()
		if err != nil {
			panic(err)
		}

		log.Println(res)
	}
}

// struct for response
type Response struct {
	Status  bool `json:"status"`
	Message any  `json:"message"`
	Data    any  `json:"data"`
}

// struct for datatable
type DataTable struct {
	Status          bool                     `json:"status"`
	Draw            int                      `json:"draw"`
	Data            []map[string]interface{} `json:"data"`
	RecordsFiltered int64                    `json:"recordsFiltered"`
	RecordsTotal    int64                    `json:"recordsTotal"`
}

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

// recover log after panic
func ErrorLog() {
	message := recover()

	if message != nil {
		fmt.Println(message)
	} else {
		fmt.Println("successfully send")
	}
}

func ArrayKey(ar map[string]interface{}) []string {

	keys := make([]string, 0, len(ar))
	for k := range ar {
		keys = append(keys, k)
	}

	return keys
}

func CreateData(c *gin.Context, table *gorm.DB, field []string) Response {

	defer ErrorLog()

	query := table
	for _, value := range field {
		check := c.Query(value)
		if check != "" {
			query.Where(value+" = ?", c.Query(value))
		}
	}

	var results []map[string]interface{}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		query.Find(&results)
	} else {
		query.Limit(limit).Find(&results)
	}

	return Response{
		Status: true,
		Data:   results,
	}
}

func CreateDataTable(c *gin.Context, table *gorm.DB, search []string) DataTable {

	defer ErrorLog()

	//MANDATORY
	draw, err := strconv.Atoi(c.PostForm("draw"))
	if err != nil {
		panic("draw not found")
	}

	limit, err := strconv.Atoi(c.PostForm("length"))
	if err != nil {
		panic("limit not found")
	}

	offset, err := strconv.Atoi(c.PostForm("start"))
	if err != nil {
		panic("offset not found")
	}

	query := table

	//WHERE
	inField := c.PostForm("in_field")
	inSearch := c.PostForm("in_search")

	if inSearch != "" {
		where := fmt.Sprintf("%s IN ?", inField)

		query.Where(where, [...]string{inSearch})
	}

	for i, val := range search {
		operator := fmt.Sprintf("tempOperator[%s]", val)
		field := fmt.Sprintf("tempSearch[%s]", val)

		value := c.PostForm(field)
		if value != "" {
			i++
			op := c.PostForm(operator)
			where := fmt.Sprintf("%s %s ?", val, op)

			query.Where(where, value)
		}

		//SEARCH VALUE
		searchBox := c.PostForm("search[value]")
		if searchBox != "" {
			if i == 0 {
				where := fmt.Sprintf("%s LIKE ?", val)
				query.Where(where, "%"+searchBox+"%")
			} else {
				where := fmt.Sprintf("%s LIKE ?", val)
				query.Or(where, "%"+searchBox+"%")
			}
		}
	}

	tempSort := c.PostForm("tempSort")

	if tempSort != "" {
		query.Order(tempSort)
	}

	var recordsTotal int64
	var results []map[string]interface{}

	query.Limit(limit).Offset(offset).Find(&results).Count(&recordsTotal)

	return  DataTable{
			Status:          true,
			Draw:            draw,
			Data:            results,
			RecordsFiltered: recordsTotal,
			RecordsTotal:    recordsTotal,
	}
}

func InsertData(db *gorm.DB, table string, data any) any {

	result := db.Table(table).Create(data).Error

	if result != nil {
		db.Rollback()
		if mysqlErr, ok := result.(*mysql.MySQLError); ok {

			return mysqlErr.Message
		} else {
			return "Error saat menyimpan data!"
		}
	}

	return nil
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

func GetRequest(c *gin.Context) []byte{

	param := c.Request.URL.Query()

	mars, err := json.Marshal(param)
	if err != nil {
		panic(err)
	}
	// convert := fmt.Sprintf(`[%s]`, data)
	// var con = []byte(convert)
	body, err := c.GetRawData()
	if err != nil {
		panic(err)
	}

	return append(mars, body...);
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