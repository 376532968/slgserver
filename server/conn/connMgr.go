package conn

import (
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"slgserver/log"
	"slgserver/net"
	"slgserver/server/pos"
	"sync"
)

var ConnMgr = Mgr{}
var cid = 0
type Mgr struct {
	cm        sync.RWMutex
	um        sync.RWMutex
	rm        sync.RWMutex

	connCache map[int]*net.WSConn
	userCache map[int]*net.WSConn
	roleCache map[int]*net.WSConn
}

func (this *Mgr) NewConn(wsSocket *websocket.Conn, needSecret bool) *net.WSConn{
	this.cm.Lock()
	defer this.cm.Unlock()

	cid++
	if this.connCache == nil {
		this.connCache = make(map[int]*net.WSConn)
	}

	if this.userCache == nil {
		this.userCache = make(map[int]*net.WSConn)
	}

	if this.roleCache == nil {
		this.roleCache = make(map[int]*net.WSConn)
	}

	c := net.NewWSConn(wsSocket, needSecret)
	c.SetProperty("cid", cid)
	this.connCache[cid] = c

	return c
}

func (this *Mgr) UserLogin(conn *net.WSConn, session string, uid int) {
	this.um.Lock()
	defer this.um.Unlock()

	oldConn, ok := this.userCache[uid]
	if ok {
		if conn != oldConn {
			log.DefaultLog.Info("rob login",
				zap.Int("uid", uid),
				zap.String("oldAddr", oldConn.Addr()),
				zap.String("newAddr", conn.Addr()))

			//这里需要通知旧端被抢登录
			oldConn.Send("robLogin", nil)
		}
	}
	this.userCache[uid] = conn
	conn.SetProperty("session", session)
	conn.SetProperty("uid", uid)
}

func (this *Mgr) UserLogout(conn *net.WSConn) {
	this.RemoveConn(conn)
}

func (this *Mgr) RoleEnter(conn *net.WSConn, rid int) {
	this.rm.Lock()
	defer this.rm.Unlock()
	conn.SetProperty("rid", rid)
	this.roleCache[rid] = conn
}

func (this *Mgr) RemoveConn(conn *net.WSConn){
	this.cm.Lock()
	cid, err := conn.GetProperty("cid")
	if err == nil {
		delete(this.connCache, cid.(int))
	}
	this.cm.Unlock()

	this.um.Lock()
	uid, err := conn.GetProperty("uid")
	if err == nil {
		delete(this.userCache, uid.(int))
	}
	this.um.Unlock()

	this.rm.Lock()
	rid, err := conn.GetProperty("rid")
	if err == nil {
		delete(this.roleCache, rid.(int))
	}
	this.rm.Unlock()

	conn.RemoveProperty("session")
	conn.RemoveProperty("uid")
	conn.RemoveProperty("role")
	conn.RemoveProperty("rid")
}

func (this *Mgr) PushByRoleId(rid int, msgName string, data interface{}) bool {
	if rid <= 0	{
		return false
	}
	this.rm.Lock()
	defer this.rm.Unlock()
	conn, ok := this.roleCache[rid]
	if ok {
		conn.Send(msgName, data)
		return true
	}else{
		return false
	}
}

func (this *Mgr) Count() int{
	this.cm.RLock()
	defer this.cm.RUnlock()

	return len(this.connCache)
}

func (this *Mgr) Push(pushSync PushSync){

	proto := pushSync.ToProto()
	rids := pushSync.BelongToRId()
	isCellView := pushSync.IsCellView()
	x, y := pushSync.Position()

	if isCellView {
		rids := pos.RPMgr.GetCellRoleIds(x, y, 5, 4)
		for _, rid := range rids {
			this.PushByRoleId(rid, pushSync.PushMsgName(), proto)
		}
	}else{
		for _, rid := range rids {
			this.PushByRoleId(rid, pushSync.PushMsgName(), proto)
		}
	}
}

func (this *Mgr) pushAll(msgName string, data interface{}) {

	this.rm.Lock()
	defer this.rm.Unlock()
	for _, conn := range this.roleCache {
		conn.Send(msgName, data)
	}
}