package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type ReserveInfo struct {
	StudentNum string `json:"studentNum"`
	Name       string `json:"name"`
	Tel        string `json:"tel"`
	Id         string `json:"id"`
	Date       string `json:"date"`
	StartTime  string `json:"startTime"`
}

const (
	DateFormat = "2006-01-02"
	TimeFormat = "2006-01-02 15:04:05"

	ServerIP      = "8.129.5.136"
	PostURLPrefix = "http://reservation.ruichengyunqin.com/api/blade-app/qywx/saveOrder?userid=%s"
	GETURLPrefix  = "http://reservation.ruichengyunqin.com/api/blade-app/qywx/getOrderTimeConfigList?groundId=%s&startDate=%s&endDate=%s&userid=%s"
)

const (
	AbleReserve = "0"
	NotReserve  = "2"
)

var GroundID = [...]string{"1298272433186332673", "1298272520994086913", "1298272615009411073", "1298272709167341570", "1298272791098875905",
	"1298273087183183874", "1298273175146127362", "1298273265650819073", "1298273399927267330", "1298273500317933570"}

func readRerserveInfo(path string) ReserveInfo {
	confFile, err := os.Open(path)
	if err != nil {
		fmt.Println("Read conf file failed!")
		os.Exit(1)
	}
	defer confFile.Close()
	decoder := json.NewDecoder(confFile)
	reserveInfo := ReserveInfo{}
	for {
		err := decoder.Decode(&reserveInfo)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Decode failed! Error:", err)
			os.Exit(1)
		}
	}
	fmt.Println("reserve information:")
	fmt.Println(reserveInfo)

	return reserveInfo
}

func main() {
	reserveInfo := readRerserveInfo("./json/conf.json")

	todayDate := time.Now()
	for todayDate.Hour() != 20 {
		fmt.Println("Do not arrive 20 Hour!")
	}

	lastDate := todayDate.Add(7 * 24 * time.Hour)
	startTimeStr := fmt.Sprintf("%s %s", reserveInfo.Date, reserveInfo.StartTime)
	startTime, err := time.Parse(TimeFormat, startTimeStr)
	fmt.Println("Reserve Start time:", startTime)
	if err != nil {
		fmt.Println("Time Parse failed! Error:", err)
		fmt.Println("Check date Whether correspond: xxxx-xx-xx, startTime correspond: xx-xx-xx ")
	}
	endTime := startTime.Add(1 * time.Hour)
	fmt.Println("Reserve End time:", endTime.String())

	diffDay := (int)(startTime.Sub(todayDate).Abs().Hours() / 24)
	hourIndex := (int)(startTime.Hour()-8) * 2

	for i, idNum := range GroundID {
		index := i + 1
		getURL := fmt.Sprintf(GETURLPrefix, idNum, todayDate.Format(DateFormat), lastDate.Format(DateFormat), reserveInfo.StudentNum)
		client := &http.Client{Timeout: 10 * time.Second}
		response, err := client.Get(getURL)
		if err != nil {
			fmt.Println("Get Url Failed! Error:", err)
		}
		var res map[string]interface{}
		body, err := io.ReadAll(response.Body)
		err = json.Unmarshal(body, &res)
		if res == nil || res["code"].(float64) != 200 || !res["success"].(bool) {
			fmt.Printf("Request Ground:%d Url: %s Error: %s", index, getURL, err)
			continue
		}
		dayTimeReserveInfo := res["data"].(map[string]interface{})["configList"].([]interface{})

		timeReserveInfo := dayTimeReserveInfo[diffDay-1].(map[string]interface{})
		timeBlockInfo := timeReserveInfo["timeBlockList"].([]interface{})
		ableReserveNum := 0
		for j := hourIndex; j < hourIndex+4; j++ {
			info := timeBlockInfo[j].(map[string]interface{})
			if status := info["status"]; status == AbleReserve {
				ableReserveNum += 1
			} else {
				break
			}
		}
		if ableReserveNum == 4 {
			postJsonFile, err := os.ReadFile("./json/post_template.json")
			if err != nil {
				fmt.Println("Read json/post template file failed! Error:", err)
				os.Exit(1)
			}
			var dataAtrr map[string]interface{}
			err = json.Unmarshal(postJsonFile, &dataAtrr)
			dataAtrr["groundId"] = idNum
			dataAtrr["groundName"] = strconv.Itoa(index) + "号场"
			dataAtrr["orderDate"] = todayDate.Format(TimeFormat)
			dataAtrr["startTime"] = startTime.Format(TimeFormat)
			dataAtrr["endTime"] = endTime.Format(TimeFormat)
			dataAtrr["tmpOrderDate"] = dataAtrr["orderDate"]
			dataAtrr["tmpStartTime"] = dataAtrr["startTime"]
			dataAtrr["tmpEndTime"] = dataAtrr["endTime"]

			postJsonFile, err = json.Marshal(dataAtrr)
			postURL := fmt.Sprintf(PostURLPrefix, reserveInfo.StudentNum)
			response, err := client.Post(postURL, "application/json", bytes.NewBuffer(postJsonFile))
			if err != nil {
				fmt.Printf("postURL %s Failed! Error: %s", postURL, err)
				fmt.Println(response)
			}
		} else {
			fmt.Printf("Ground %d has been reserved in %s\n", index, startTimeStr)
		}

	}

}
