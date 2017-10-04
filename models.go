package main

import (
	"time"
)

// Account user account
type Account struct {
	ID        string
	Name      string
	SessionID string
}

// Course user course
type Course struct {
	ID         string
	Name       string
	SchoolYear int
	Semester   int
	// course pages e.g. homework,notice ...
	// page title -> page id
	Pages map[string]string
}

// GetPageID get couse page id by page title return (id,has)
func (course *Course) GetPageID(pageTile string) (string, bool) {
	if course.Pages == nil {
		return "", false
	}
	id, has := course.Pages[pageTile]
	return id, has
}

// CourseList course list which user subscribe
type CourseList struct {
	UserID  string
	Courses []*Course
}

// Add add course to course list
func (clist *CourseList) Add(course *Course) {
	clist.Courses = append(clist.Courses, course)
}

func (clist *CourseList) filterCourse(fun func(*Course) bool) []*Course {
	ret := make([]*Course, 0)
	for _, course := range clist.Courses {
		if fun(course) {
			ret = append(ret, course)
		}
	}
	return ret
}

// GetByShcoolYearAndSemester get courses by shcool year and semester
func (clist *CourseList) GetByShcoolYearAndSemester(shcoolYear, semester int) []*Course {
	return clist.filterCourse(func(c *Course) bool {
		return c.SchoolYear == shcoolYear && c.Semester == semester
	})
}

// HomeWrokStatus home wrok status e.g. unsubmit,submited...
type HomeWrokStatus uint

const (
	Submitted HomeWrokStatus = iota + 1
	UnSubmit
	Expired
)

// HomeWrok user's home work
type HomeWrok struct {
	Title         string
	Status        HomeWrokStatus
	StatusMessage string
	StartTime     time.Time
	DueTime       time.Time
}
