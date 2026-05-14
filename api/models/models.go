package models

import "time"

type Radcheck struct {
	ID        uint   `gorm:"primaryKey;column:id" json:"id"`
	Username  string `gorm:"column:username;size:64;index" json:"username"`
	Attribute string `gorm:"column:attribute;size:64" json:"attribute"`
	Op        string `gorm:"column:op;size:2" json:"op"`
	Value     string `gorm:"column:value;size:253" json:"value"`
}

func (Radcheck) TableName() string { return "radcheck" }

type Radreply struct {
	ID        uint   `gorm:"primaryKey;column:id" json:"id"`
	Username  string `gorm:"column:username;size:64;index" json:"username"`
	Attribute string `gorm:"column:attribute;size:64" json:"attribute"`
	Op        string `gorm:"column:op;size:2" json:"op"`
	Value     string `gorm:"column:value;size:253" json:"value"`
}

func (Radreply) TableName() string { return "radreply" }

type Radgroupcheck struct {
	ID        uint   `gorm:"primaryKey;column:id" json:"id"`
	Groupname string `gorm:"column:groupname;size:64;index" json:"groupname"`
	Attribute string `gorm:"column:attribute;size:64" json:"attribute"`
	Op        string `gorm:"column:op;size:2" json:"op"`
	Value     string `gorm:"column:value;size:253" json:"value"`
}

func (Radgroupcheck) TableName() string { return "radgroupcheck" }

type Radgroupreply struct {
	ID        uint   `gorm:"primaryKey;column:id" json:"id"`
	Groupname string `gorm:"column:groupname;size:64;index" json:"groupname"`
	Attribute string `gorm:"column:attribute;size:64" json:"attribute"`
	Op        string `gorm:"column:op;size:2" json:"op"`
	Value     string `gorm:"column:value;size:253" json:"value"`
}

func (Radgroupreply) TableName() string { return "radgroupreply" }

type Radusergroup struct {
	ID        uint   `gorm:"primaryKey;column:id" json:"id"`
	Username  string `gorm:"column:username;size:64;index" json:"username"`
	Groupname string `gorm:"column:groupname;size:64" json:"groupname"`
	Priority  int    `gorm:"column:priority" json:"priority"`
}

func (Radusergroup) TableName() string { return "radusergroup" }

type Radacct struct {
	RadAcctID           uint64     `gorm:"primaryKey;column:radacctid" json:"radacctid"`
	AcctSessionID       string     `gorm:"column:acctsessionid;size:64" json:"acctsessionid"`
	AcctUniqueID        string     `gorm:"column:acctuniqueid;size:32;uniqueIndex" json:"acctuniqueid"`
	Username            string     `gorm:"column:username;size:64;index" json:"username"`
	Realm               *string    `gorm:"column:realm;size:64" json:"realm,omitempty"`
	NASIPAddress        string     `gorm:"column:nasipaddress;size:15;index" json:"nasipaddress"`
	NASPortID           *string    `gorm:"column:nasportid;size:32" json:"nasportid,omitempty"`
	NASPortType         *string    `gorm:"column:nasporttype;size:32" json:"nasporttype,omitempty"`
	AcctStartTime       *time.Time `gorm:"column:acctstarttime;index" json:"acctstarttime,omitempty"`
	AcctUpdateTime      *time.Time `gorm:"column:acctupdatetime" json:"acctupdatetime,omitempty"`
	AcctStopTime        *time.Time `gorm:"column:acctstoptime;index" json:"acctstoptime,omitempty"`
	AcctInterval        *int       `gorm:"column:acctinterval" json:"acctinterval,omitempty"`
	AcctSessionTime     *int       `gorm:"column:acctsessiontime" json:"acctsessiontime,omitempty"`
	AcctAuthentic       *string    `gorm:"column:acctauthentic;size:32" json:"acctauthentic,omitempty"`
	ConnectInfoStart    *string    `gorm:"column:connectinfo_start;size:128" json:"connectinfo_start,omitempty"`
	ConnectInfoStop     *string    `gorm:"column:connectinfo_stop;size:128" json:"connectinfo_stop,omitempty"`
	AcctInputOctets     *int64     `gorm:"column:acctinputoctets" json:"acctinputoctets,omitempty"`
	AcctOutputOctets    *int64     `gorm:"column:acctoutputoctets" json:"acctoutputoctets,omitempty"`
	CalledStationID     string     `gorm:"column:calledstationid;size:50" json:"calledstationid"`
	CallingStationID    string     `gorm:"column:callingstationid;size:50" json:"callingstationid"`
	AcctTerminateCause  string     `gorm:"column:acctterminatecause;size:32" json:"acctterminatecause"`
	ServiceType         *string    `gorm:"column:servicetype;size:32" json:"servicetype,omitempty"`
	FramedProtocol      *string    `gorm:"column:framedprotocol;size:32" json:"framedprotocol,omitempty"`
	FramedIPAddress     string     `gorm:"column:framedipaddress;size:15" json:"framedipaddress"`
	FramedIPv6Address   string     `gorm:"column:framedipv6address;size:45" json:"framedipv6address,omitempty"`
	FramedIPv6Prefix    string     `gorm:"column:framedipv6prefix;size:45" json:"framedipv6prefix,omitempty"`
	FramedInterfaceID   string     `gorm:"column:framedinterfaceid;size:44" json:"framedinterfaceid,omitempty"`
	DelegatedIPv6Prefix string     `gorm:"column:delegatedipv6prefix;size:45" json:"delegatedipv6prefix,omitempty"`
	Class               *string    `gorm:"column:class;size:64" json:"class,omitempty"`
}

func (Radacct) TableName() string { return "radacct" }

type Radpostauth struct {
	ID       uint      `gorm:"primaryKey;column:id" json:"id"`
	Username string    `gorm:"column:username;size:64;index" json:"username"`
	Pass     string    `gorm:"column:pass;size:64" json:"pass"`
	Reply    string    `gorm:"column:reply;size:32" json:"reply"`
	AuthDate time.Time `gorm:"column:authdate" json:"authdate"`
	Class    *string   `gorm:"column:class;size:64;index" json:"class,omitempty"`
}

func (Radpostauth) TableName() string { return "radpostauth" }

type NAS struct {
	ID          uint    `gorm:"primaryKey;column:id" json:"id"`
	NASName     string  `gorm:"column:nasname;size:128;index" json:"nasname"`
	ShortName   *string `gorm:"column:shortname;size:32" json:"shortname,omitempty"`
	Type        *string `gorm:"column:type;size:30" json:"type,omitempty"`
	Ports       *int    `gorm:"column:ports" json:"ports,omitempty"`
	Secret      string  `gorm:"column:secret;size:60" json:"-"`
	Server      *string `gorm:"column:server;size:64" json:"server,omitempty"`
	Community   *string `gorm:"column:community;size:50" json:"community,omitempty"`
	Description *string `gorm:"column:description;size:200" json:"description,omitempty"`
}

func (NAS) TableName() string { return "nas" }

// ApiKey — tabel API key + scope (read|write|admin).
// Bukan bagian schema FreeRADIUS; di-AutoMigrate oleh api saat startup.
type ApiKey struct {
	ID         uint       `gorm:"primaryKey;column:id" json:"id"`
	Name       string     `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	KeyHash    string     `gorm:"column:key_hash;size:64;uniqueIndex" json:"-"`
	Scope      string     `gorm:"column:scope;size:16;not null" json:"scope"`
	Enabled    bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	Notes      *string    `gorm:"column:notes;size:200" json:"notes,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
}

func (ApiKey) TableName() string { return "api_keys" }

// ApiAuditLog — log per request /api/* (untuk audit & forensic).
type ApiAuditLog struct {
	ID         uint64    `gorm:"primaryKey;column:id" json:"id"`
	Timestamp  time.Time `gorm:"column:timestamp;index" json:"timestamp"`
	APIKeyName string    `gorm:"column:api_key_name;size:64;index" json:"api_key_name"`
	Method     string    `gorm:"column:method;size:10" json:"method"`
	Path       string    `gorm:"column:path;size:512" json:"path"`
	Status     int       `gorm:"column:status" json:"status"`
	IP         string    `gorm:"column:ip;size:45" json:"ip"`
	UserAgent  string    `gorm:"column:user_agent;size:256" json:"user_agent"`
	DurationMs int       `gorm:"column:duration_ms" json:"duration_ms"`
}

func (ApiAuditLog) TableName() string { return "api_audit_log" }

// Radippool — milik FreeRADIUS sqlippool module (schema dari initdb/03-ippool-schema.sql).
// Tidak di-AutoMigrate; struktur harus selalu match FreeRADIUS upstream.
type Radippool struct {
	ID               uint      `gorm:"primaryKey;column:id" json:"id"`
	PoolName         string    `gorm:"column:pool_name;size:30;index" json:"pool_name"`
	FramedIPAddress  string    `gorm:"column:framedipaddress;size:15;index" json:"framedipaddress"`
	NASIPAddress     string    `gorm:"column:nasipaddress;size:15" json:"nasipaddress"`
	CalledStationID  string    `gorm:"column:calledstationid;size:30" json:"calledstationid"`
	CallingStationID string    `gorm:"column:callingstationid;size:30" json:"callingstationid"`
	ExpiryTime       time.Time `gorm:"column:expiry_time" json:"expiry_time"`
	Username         string    `gorm:"column:username;size:64" json:"username"`
	PoolKey          string    `gorm:"column:pool_key;size:30" json:"pool_key"`
}

func (Radippool) TableName() string { return "radippool" }

// Webhook — subscriber event. Cursor tracks last delivered ID per event type.
type Webhook struct {
	ID           uint       `gorm:"primaryKey;column:id" json:"id"`
	Name         string     `gorm:"column:name;size:64;uniqueIndex" json:"name"`
	URL          string     `gorm:"column:url;size:512" json:"url"`
	Secret       string     `gorm:"column:secret;size:64" json:"-"`
	Event        string     `gorm:"column:event;size:32;index" json:"event"` // session_stop|session_start|auth_accept|auth_reject
	Enabled      bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	Cursor       uint64     `gorm:"column:cursor;default:0" json:"cursor"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	LastFiredAt  *time.Time `gorm:"column:last_fired_at" json:"last_fired_at,omitempty"`
	FailureCount int        `gorm:"column:failure_count;default:0" json:"failure_count"`
}

func (Webhook) TableName() string { return "api_webhooks" }

// WebhookDelivery — log per attempt firing webhook.
type WebhookDelivery struct {
	ID         uint64    `gorm:"primaryKey;column:id" json:"id"`
	WebhookID  uint      `gorm:"column:webhook_id;index" json:"webhook_id"`
	Event      string    `gorm:"column:event;size:32" json:"event"`
	URL        string    `gorm:"column:url;size:512" json:"url"`
	StatusCode int       `gorm:"column:status_code" json:"status_code"`
	Error      string    `gorm:"column:error;size:512" json:"error,omitempty"`
	Timestamp  time.Time `gorm:"column:timestamp;index" json:"timestamp"`
	DurationMs int       `gorm:"column:duration_ms" json:"duration_ms"`
}

func (WebhookDelivery) TableName() string { return "api_webhook_deliveries" }
