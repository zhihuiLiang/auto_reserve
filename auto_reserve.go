package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

type ReserveInfo struct {
	StudentNum string `json:"studentNum"`
	Name       string `json:"name"`
	Tel        string `json:"tel"`
	Id         string `json:"id"`
}

type AbleReserveInfo struct {
	GroundId  int
	HourIndex int // 3--> 9:30, 13--> 14:30, 24-->8:00
	LastTime  int
}

type AbleReserveInfoArr []AbleReserveInfo

func (info AbleReserveInfoArr) Len() int { return len(info) }
func (info AbleReserveInfoArr) Less(i, j int) bool {
	if info[i].LastTime > info[j].LastTime {
		return true
	} else {
		if info[i].HourIndex < info[j].HourIndex {
			return true
		}
		return false
	}
}
func (info AbleReserveInfoArr) Swap(i, j int) {
	info[i], info[j] = info[j], info[i]
}

const (
	DateFormat = "2006-01-02"
	TimeFormat = "2006-01-02 15:04:05"

	ServerIP      = "8.129.5.136"
	PostURLPrefix = "http://reservation.ruichengyunqin.com/api/blade-app/qywx/saveOrder?userid=%s"
	GETURLPrefix  = "http://reservation.ruichengyunqin.com/api/blade-app/qywx/getOrderTimeConfigList?groundId=%s&startDate=%s&endDate=%s&userid=%s"

	MaxReserveIndex = 27
)

const (
	AbleReserve = "1"
	NotReserve  = "2"
)

var GroundID = [...]string{"1298272433186332673", "1298272520994086913", "1298272615009411073", "1298272709167341570", "1298272791098875905",
											"1298273087183183874", "1298273175146127362", "1298273265650819073", "1298273399927267330", "1298273500317933570"}
var IgnoreReserveIndex = map[int]bool{8: true, 9: true, 10: true, 11: true, 12: true} //忽略中午时间段

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
		if diffHour := 20 - time.Now().Hour() - 1; diffHour > 0 {
			fmt.Println("Sleep Hour:", diffHour)
			time.Sleep(time.Duration(diffHour) * time.Hour)
		}
		if diffMinute := 59 - time.Now().Minute(); diffMinute > 0 {
			fmt.Println("Sleep Minute:", diffMinute)
			time.Sleep(time.Duration(diffMinute) * time.Minute)
		}
		fmt.Println("Do not arrive 20:00 Hour! Cur Hour:", todayDate.Hour())
		todayDate = time.Now()
	}
	diffDay := 0
	if todayDate.Hour() < 20 {
		diffDay = 6
	} else {
		diffDay = 7
	}

	lastDate := todayDate.Add(time.Duration(diffDay*24) * time.Hour)
	reserveDate := todayDate.Add(time.Duration(diffDay*24) * time.Hour)
	fmt.Println("Reserve Start time:", reserveDate)

	client := &http.Client{Timeout: 10 * time.Second}
	for i, idNum := range GroundID {
		var ableReserveInfoArr AbleReserveInfoArr
		fmt.Println("Request for Ground: ", i+1)
		index := i + 1
		getURL := fmt.Sprintf(GETURLPrefix, idNum, todayDate.Format(DateFormat), lastDate.Format(DateFormat), reserveInfo.StudentNum)
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

		timeReserveInfo := dayTimeReserveInfo[diffDay].(map[string]interface{})
		timeBlockInfo := timeReserveInfo["timeBlockList"].([]interface{})
		ableReserveNum := 0

		startIndex := 22 //从7:00开始预约
		if int(todayDate.Weekday()) >= 6 {
			startIndex = 3 //从9:30开始预约
		}
		for j := startIndex; j < MaxReserveIndex; j++ {
			if _, ok := IgnoreReserveIndex[j]; ok {
				if ableReserveNum > 0 {
					ableReserveInfo := AbleReserveInfo{index, j - ableReserveNum, ableReserveNum}
					ableReserveInfoArr = append(ableReserveInfoArr, ableReserveInfo)
					ableReserveNum = 0
				}
				continue
			}

			info := timeBlockInfo[j].(map[string]interface{})
			status := info["status"]
			fmt.Println("time info:", info)
			if (status == AbleReserve) && (ableReserveNum < 4) {
				ableReserveNum += 1
			} else if ableReserveNum > 1 { //预约半个小时以上的时间段
				ableReserveInfo := AbleReserveInfo{index, j - ableReserveNum, ableReserveNum}
				ableReserveInfoArr = append(ableReserveInfoArr, ableReserveInfo)
				ableReserveNum = 0
			} else {
				ableReserveNum = 0
			}
		}
		if len(ableReserveInfoArr) > 0 {
			sort.Sort(ableReserveInfoArr)
			first := ableReserveInfoArr[0]
			beginTimeStr := reserveDate.Format(DateFormat) + " " + "08:00:00"
			beginTime, _ := time.Parse(TimeFormat, beginTimeStr)
			reserveST := beginTime.Add(time.Duration(first.HourIndex*60/2) * time.Minute)
			reserveSTStr := reserveST.Format(TimeFormat)
			reserveET := reserveST.Add(time.Duration(first.LastTime*60/2) * time.Minute)
			reserveETStr := reserveET.Format(TimeFormat)
			fmt.Println("Reserve Start Time:", reserveSTStr)
			fmt.Println("Reserve End Time:", reserveETStr)

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
			dataAtrr["startTime"] = reserveSTStr
			dataAtrr["endTime"] = reserveETStr
			dataAtrr["tmpOrderDate"] = dataAtrr["orderDate"]
			dataAtrr["tmpStartTime"] = dataAtrr["startTime"]
			dataAtrr["tmpEndTime"] = dataAtrr["endTime"]

			postJsonFile, err = json.Marshal(dataAtrr)
			postURL := fmt.Sprintf(PostURLPrefix, reserveInfo.StudentNum)
			responsePost, err := client.Post(postURL, "application/json", bytes.NewBuffer(postJsonFile))
			if err != nil {
				fmt.Printf("postURL %s Failed! Error: %s", postURL, err)
			} else {
				fmt.Println("postURL:", postURL)
				fmt.Println("post information:")
				fmt.Println(dataAtrr)
				fmt.Println(responsePost)
			}

			if first.HourIndex == 4 {
				break
			}
		} else {
			fmt.Printf("Ground %d has been reserved!\n", index)
		}
	}

}
