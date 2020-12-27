// +build aix linux darwin dragonfly freebsd openbsd netbsd solaris

package proc

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type ownership struct {
	User string
	Group string
}

// userMap and groupMap caches UID and GID lookups for performance reasons.
// The downside is that renaming uname or gname by the OS never takes effect.
var userMap, groupMap sync.Map // map[int]string

func statUnix(fi os.FileInfo) (*ownership, error) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("syscall")
	}

	uid := int(sys.Uid)
	gid := int(sys.Gid)
	uname := ""
	gname := ""

	if u, ok := userMap.Load(uid); ok {
		uname = u.(string)
	} else if u, err := user.LookupId(strconv.Itoa(uid)); err == nil {
		uname = u.Username
		userMap.Store(uid, uname)
	}
	if g, ok := groupMap.Load(gid); ok {
		gname = g.(string)
	} else if g, err := user.LookupGroupId(strconv.Itoa(gid)); err == nil {
		gname = g.Name
		groupMap.Store(gid, gname)
	}

	return &ownership{uname, gname}, nil
}

func statAtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Atim.Unix())
}

func statCtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Ctim.Unix())
}

type ownershipChecker struct {}

func (d ownershipChecker) prepareCheck(fqn string, fi os.FileInfo) (interface{}, error) {
	owner, err := statUnix(fi)
	if err != nil {
		return nil, fmt.Errorf("retreive ownership:%v", err)
	}
	return owner, nil
}

func (d ownershipChecker) executeCheck(fqn string, data interface{}, fi os.FileInfo) error {
	expectedData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("data corrupt")
	}
	usr, ok := expectedData["User"]
	if !ok {
		return fmt.Errorf("data corrupt")
	}
	expectedOwner := &ownership{}
	expectedOwner.User, ok = usr.(string)
	if !ok {
		return fmt.Errorf("data corrupt")
	}
	group, ok := expectedData["Group"]
	if !ok {
		return fmt.Errorf("data corrupt")
	}
	expectedOwner.Group, ok = group.(string)
	if !ok {
		return fmt.Errorf("data corrupt")
	}

	actualOwner, err := statUnix(fi)
	if err != nil {
		return fmt.Errorf("retreive ownership:%v", err)
	}

	if (expectedOwner.User != actualOwner.User) || (expectedOwner.Group != actualOwner.Group) {
		return fmt.Errorf("expected %s:%s actual %s:%s",
			expectedOwner.User, expectedOwner.Group,
			actualOwner.User, actualOwner.Group)
	}
	return nil
}
