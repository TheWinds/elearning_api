package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/cihub/seelog"
	"github.com/imroc/req"
)

const (
	loginURL              = "http://elearning.usx.edu.cn/portal/relogin"
	homeURL               = "http://elearning.usx.edu.cn/portal"
	getCoursePagesBaseURL = "http://elearning.usx.edu.cn/direct/site/%s/pages.json?locale=zh-CN"
	getHomeWorksBaseURL   = "http://elearning.usx.edu.cn/portal/tool/%s?panel=Main"
)

// NetError net error
type NetError struct {
	Code    int
	Message string
}

func (e *NetError) Error() string {
	return e.Message
}

func parseUserName(rawUserInfo string) string {
	if rawUserInfo == "" {
		return ""
	}
	return strings.Split(rawUserInfo, "(")[0]
}

func getCookieValue(name string, cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if name == cookie.Name {
			return cookie.Value
		}
	}
	return ""
}

// Login login to elearning and return user account(contains session and  username)
func Login(userID, password string) (*Account, error) {
	log.Info("user:", userID, " start login")
	// save cookie
	jar, _ := cookiejar.New(nil)
	req.Client().Jar = jar
	// do login
	r, err := req.Post(loginURL, req.Param{
		"eid":    userID,
		"pw":     password,
		"submit": "%E7%99%BB%E5%BD%95",
	}, req.Header{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't fetch webpage"}
	}
	// parse
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Bytes()))
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't parse document"}
	}
	loginErrMsg := doc.Find("form div.alertMessage").Text()
	if loginErrMsg != "" {
		return nil, &NetError{Code: 400, Message: "username or password error"}
	}
	account := &Account{ID: userID}
	// get user account infomation
	account.Name = parseUserName(doc.Find("#loginUser").Text())
	// get session id
	u, _ := url.Parse(loginURL)
	account.SessionID = getCookieValue("JSESSIONID", jar.Cookies(u))
	log.Info("user:", userID, " login over\n", *account)
	return account, nil
}

// bind session to cookie
func makeCookie(session string) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = "JSESSIONID"
	cookie.Value = session
	return cookie
}

func parseCourseYearAndSemester(rawStr string) (int, int) {
	pattern, _ := regexp.Compile("[0-9]+")
	nums := pattern.FindAllString(rawStr, -1)
	if len(nums) != 3 {
		return 0, 0
	}
	semester, err := strconv.Atoi(nums[2])
	if err != nil {
		log.Error("parseCourseSemester", err)
		return 0, 0
	}
	year, err := strconv.Atoi(nums[0])
	if err != nil {
		log.Error("parseCourseSchoolYear", err)
		return 0, 0
	}
	return year, semester

}

func parseCourseID(url string) string {
	lastIndex := strings.LastIndex(url, "/")
	if lastIndex == -1 {
		return ""
	}
	return url[lastIndex+1:]
}

// GetUserCourseList getUserCourseList
func GetUserCourseList(session string) (*CourseList, error) {
	log.Info("start getUserCourseList session: ", session)
	r, err := req.Get(homeURL, makeCookie(session))
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't fetch webpage"}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Bytes()))
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't parse document"}
	}
	courseList := new(CourseList)
	// parse course
	doc.
		Find(".otherSitesCategorList .fullTitle").
		Each(func(_ int, sel *goquery.Selection) {
			course := new(Course)
			// course name
			course.Name = sel.Text()

			// school-year and semester
			rawss := sel.Parent().Parent().Parent().Prev().Text()
			course.SchoolYear, course.Semester = parseCourseYearAndSemester(rawss)
			// course id
			if url, has := sel.Parent().Attr("href"); has {
				course.ID = parseCourseID(url)
			}
			courseList.Add(course)
		})
	// getCoursePages async
	wg := new(sync.WaitGroup)
	for _, course := range courseList.Courses {
		wg.Add(1)
		go getCoursePages(session, course, wg)
	}
	wg.Wait()
	log.Info("getUserCourseList over")
	return courseList, nil
}

type coursePage struct {
	Tools []coursePageTools `json:"tools"`
}
type coursePageTools struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type coursePagesResp []*coursePage

// getCoursePages
func getCoursePages(session string, course *Course, wg *sync.WaitGroup) {
	defer wg.Done()
	reqURL := fmt.Sprintf(getCoursePagesBaseURL, course.ID)
	r, err := req.Get(reqURL, makeCookie(session))
	if err != nil {
		log.Error(err)
		return
	}
	coursePages := make(coursePagesResp, 0)
	err = r.ToJSON(&coursePages)
	if err != nil {
		panic(err)
	}
	pages := make(map[string]string)
	for _, page := range coursePages {
		for _, tool := range page.Tools {
			//log.Debug(tool.Title, tool.ID)
			pages[tool.Title] = tool.ID
		}
	}
	course.Pages = pages
}

// parseTime parse homework page time to unix time
func parseTime(rawtime string) time.Time {
	rawtime = strings.Replace(rawtime, "下午", "PM ", -1)
	rawtime = strings.Replace(rawtime, "上午", "AM ", -1)
	tm, err := time.Parse("2006-1-2 PM 3:04", rawtime)
	if err != nil {
		log.Error(err)
		return time.Now().AddDate(-10, 0, 0)
	}
	return tm

}

func parseHomeWorkStatus(rawStatus string, dueTime time.Time) HomeWrokStatus {
	if strings.HasPrefix(rawStatus, "已提交") {
		return Submitted
	}
	if time.Now().Unix() > dueTime.Unix() {
		return Expired
	}
	return UnSubmit
}

// GetCourseHomeWorks
func GetCourseHomeWorks(session, pageID string) ([]*HomeWrok, error) {
	reqURL := fmt.Sprintf(getHomeWorksBaseURL, pageID)
	r, err := req.Get(reqURL, makeCookie(session))
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't fetch webpage"}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Bytes()))
	if err != nil {
		log.Error(err)
		return nil, &NetError{Code: 500, Message: "net error,can't parse document"}
	}
	homeWorks := make([]*HomeWrok, 0)
	doc.Find("table tr").
		Each(func(_ int, row *goquery.Selection) {
			cols := row.Find("td")
			title := cols.Eq(1).Find("a").Text()
			if title == "" {
				return
			}
			status := strings.TrimSpace(cols.Eq(2).Text())
			startTime := strings.TrimSpace(cols.Eq(3).Text())
			dueTime := strings.TrimSpace(cols.Eq(4).Find("span").Text())
			homeWork := &HomeWrok{
				Title:         title,
				StatusMessage: status,
				StartTime:     parseTime(startTime),
				DueTime:       parseTime(dueTime),
			}
			homeWork.Status = parseHomeWorkStatus(status, homeWork.DueTime)
			homeWorks = append(homeWorks, homeWork)
			log.Info(homeWork)
		})
	return homeWorks, nil
}
