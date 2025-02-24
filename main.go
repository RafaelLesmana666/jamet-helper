package jamethelper

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)


type Jamet struct {
	config map[string]*gorm.DB
}

func NewJamet(config map[string]*gorm.DB) *Jamet {
	return &Jamet{
		config:     config,
	}
}

func (met Jamet) GetData(table string, connection string) *gorm.DB {

	return met.config[connection].Table(table)
}

func (met Jamet) Connection(conn string) *gorm.DB {
	db := met.config[conn]

	return db.Begin()
}


type Response struct {
	Status  bool `json:"status"`
	Message any  `json:"message"`
	Data    any  `json:"data"`
}

type DataTable struct {
	Status          bool                     `json:"status"`
	Draw            int                      `json:"draw"`
	Data            []map[string]interface{} `json:"data"`
	RecordsFiltered int64                    `json:"recordsFiltered"`
	RecordsTotal    int64                    `json:"recordsTotal"`
}

func PrintJSON(c *gin.Context, response Response) {
	c.Render(http.StatusOK, render.JSON{Data: response})
}

func EPrintJSON(c *gin.Context, response Response) {
	c.Render(http.StatusBadRequest, render.JSON{Data: response})
}

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

func CreateData(c *gin.Context, table *gorm.DB, field []string) {

	defer ErrorLog()

	query := table
	for _, value := range field {
		fmt.Println(value)
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

	fmt.Println(query);

	PrintJSON(c, Response{
		Status: true,
		Data:   results,
	})
}

func CreateDataTable(c *gin.Context, table *gorm.DB, search []string) {

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
	c.Render(http.StatusOK, render.JSON{Data: DataTable{
		Status:          true,
		Draw:            draw,
		Data:            results,
		RecordsFiltered: recordsTotal,
		RecordsTotal:    recordsTotal,
	}})
}

func InsertData(c *gin.Context, db *gorm.DB, table string, data any) any {

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
		if strings.Contains(v,str) {
			index := fmt.Sprintf("%d", i)
			return true, index
		}
	}
	return false, ""
}

func Validation(request map[string]interface{}, format map[string]map[string]string) string {

	var errMessage []string
	message := format["message"]
	alias   := format["alias"]
	for key, value := range format["field"] {
		
		var contain bool
		var index string
		cond := strings.Split(value, "|")
		formData := request[key].(string)

		contain, _ = Contains(cond, "required")
		if contain && formData == "" {
			
			if len(message) > 0 && message[key] != "" {
				errMessage = append(errMessage,message[key])
			} else {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage,fmt.Sprintf("%s perlu diisi", alias[key]))
				} else {
					errMessage = append(errMessage,fmt.Sprintf("%s perlu diisi", key))
				}
			}

			continue
		}

		//min
		contain, index = Contains(cond, "min")
		if contain {

			i, err := strconv.Atoi(index); if err != nil {
				errMessage = append(errMessage,"Error pada saat validasi")
				break;
			}

			arr := strings.Split(cond[i],":")
			min, err := strconv.Atoi(arr[1]); if err != nil {
				errMessage = append(errMessage,"Error pada saat validasi")
				break;
			}

			if len(formData) < min {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage,fmt.Sprintf("Panjang %s kurang dari %d", alias[key],min))
				} else {
					errMessage = append(errMessage,fmt.Sprintf("Panjang %s kurang dari %d", key,min))
				}
				continue
			}
		}

		//max
		contain, index = Contains(cond, "max")
		if contain {

			i, err := strconv.Atoi(index); if err != nil {
				errMessage = append(errMessage,"Error pada saat validasi")
				break;
			}

			arr := strings.Split(cond[i],":")
			max, err := strconv.Atoi(arr[1]); if err != nil {
				errMessage = append(errMessage,"Error pada saat validasi")
				break;
			}

			if len(formData) > max {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage,fmt.Sprintf("Panjang %s lebih dari %d", alias[key],max))
				} else {
					errMessage = append(errMessage,fmt.Sprintf("Panjang %s lebih dari %d", key,max))
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

func Converter(c *gin.Context) map[string]interface{} {

	req := c.PostForm("data")
	convert := fmt.Sprintf(`[%s]`,req)
	var jsonBlob = []byte(convert)
	var objmap []map[string]interface{}
	if err := json.Unmarshal(jsonBlob, &objmap); err != nil {
	    log.Fatal(err)
	}

	return objmap[0]
}
