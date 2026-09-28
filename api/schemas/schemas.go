package schemas

import "time"

type AttributeIn struct {
	Attribute string `json:"attribute" example:"Session-Timeout"`
	Op        string `json:"op" example:":="`
	Value     string `json:"value" example:"3600"`
}

type AttributeOut struct {
	ID        uint   `json:"id"`
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Value     string `json:"value"`
}

type AttributeUpdate struct {
	Attribute *string `json:"attribute,omitempty"`
	Op        *string `json:"op,omitempty"`
	Value     *string `json:"value,omitempty"`
}

type UserCreate struct {
	Username        string        `json:"username" example:"alice"`
	Password        string        `json:"password" example:"s3cret"`
	PasswordAttr    string        `json:"password_attr" example:"Cleartext-Password"`
	CheckAttributes []AttributeIn `json:"check_attributes,omitempty"`
	ReplyAttributes []AttributeIn `json:"reply_attributes,omitempty"`
	Groups          []string      `json:"groups,omitempty"`
}

type UserOut struct {
	Username        string         `json:"username"`
	CheckAttributes []AttributeOut `json:"check_attributes"`
	ReplyAttributes []AttributeOut `json:"reply_attributes"`
	Groups          []string       `json:"groups"`
}

type UserPasswordUpdate struct {
	Password     string `json:"password" example:"newpass"`
	PasswordAttr string `json:"password_attr" example:"Cleartext-Password"`
}

type GroupCreate struct {
	Groupname       string        `json:"groupname" example:"wifi"`
	CheckAttributes []AttributeIn `json:"check_attributes,omitempty"`
	ReplyAttributes []AttributeIn `json:"reply_attributes,omitempty"`
}

type GroupOut struct {
	Groupname       string         `json:"groupname"`
	CheckAttributes []AttributeOut `json:"check_attributes"`
	ReplyAttributes []AttributeOut `json:"reply_attributes"`
}

type NASCreate struct {
	NASName     string `json:"nasname" example:"192.168.1.1"`
	ShortName   string `json:"shortname,omitempty" example:"mainrouter"`
	Type        string `json:"type,omitempty" example:"other"`
	Ports       *int   `json:"ports,omitempty"`
	Secret      string `json:"secret" example:"testing123"`
	Server      string `json:"server,omitempty"`
	Community   string `json:"community,omitempty"`
	Description string `json:"description,omitempty"`
}

type NASUpdate struct {
	NASName     *string `json:"nasname,omitempty"`
	ShortName   *string `json:"shortname,omitempty"`
	Type        *string `json:"type,omitempty"`
	Ports       *int    `json:"ports,omitempty"`
	Secret      *string `json:"secret,omitempty"`
	Server      *string `json:"server,omitempty"`
	Community   *string `json:"community,omitempty"`
	Description *string `json:"description,omitempty"`
}

type AccountingSession struct {
	RadAcctID           uint64     `json:"radacctid"`
	AcctSessionID       string     `json:"acctsessionid"`
	AcctUniqueID        string     `json:"acctuniqueid"`
	Username            string     `json:"username"`
	Realm               *string    `json:"realm,omitempty"`
	NASIPAddress        string     `json:"nasipaddress"`
	NASPortID           *string    `json:"nasportid,omitempty"`
	NASPortType         *string    `json:"nasporttype,omitempty"`
	AcctStartTime       *time.Time `json:"acctstarttime,omitempty"`
	AcctUpdateTime      *time.Time `json:"acctupdatetime,omitempty"`
	AcctStopTime        *time.Time `json:"acctstoptime,omitempty"`
	AcctSessionTime     *int       `json:"acctsessiontime,omitempty"`
	AcctAuthentic       *string    `json:"acctauthentic,omitempty"`
	ConnectInfoStart    *string    `json:"connectinfo_start,omitempty"`
	ConnectInfoStop     *string    `json:"connectinfo_stop,omitempty"`
	AcctInputOctets     *int64     `json:"acctinputoctets,omitempty"`
	AcctOutputOctets    *int64     `json:"acctoutputoctets,omitempty"`
	CalledStationID     string     `json:"calledstationid"`
	CallingStationID    string     `json:"callingstationid"`
	AcctTerminateCause  string     `json:"acctterminatecause"`
	ServiceType         *string    `json:"servicetype,omitempty"`
	FramedProtocol      *string    `json:"framedprotocol,omitempty"`
	FramedIPAddress     string     `json:"framedipaddress,omitempty"`
	FramedIPv6Address   string     `json:"framedipv6address,omitempty"`
	FramedIPv6Prefix    string     `json:"framedipv6prefix,omitempty"`
	DelegatedIPv6Prefix string     `json:"delegatedipv6prefix,omitempty"`
	Class               *string    `json:"class,omitempty"`
}

type UserUsage struct {
	Username            string `json:"username"`
	SessionCount        int64  `json:"session_count"`
	TotalSessionSeconds int64  `json:"total_session_seconds"`
	InputBytes          int64  `json:"input_bytes"`
	OutputBytes         int64  `json:"output_bytes"`
}

type PostAuthOut struct {
	ID       uint      `json:"id"`
	Username string    `json:"username"`
	Pass     string    `json:"pass"`
	Reply    string    `json:"reply"`
	AuthDate time.Time `json:"authdate"`
	Class    *string   `json:"class,omitempty"`
}

type DisconnectRequest struct {
	Port    int `json:"port,omitempty" example:"3799"`
	Timeout int `json:"timeout_ms,omitempty" example:"3000"`
}

type DisconnectResponse struct {
	Status        string `json:"status"`
	Code          string `json:"code"`
	NASIPAddress  string `json:"nasipaddress"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AcctSessionID string `json:"acctsessionid"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ---- API keys ----

type ApiKeyCreate struct {
	Name  string  `json:"name" example:"web-portal"`
	Scope string  `json:"scope" example:"write"`
	Notes *string `json:"notes,omitempty"`
}

type ApiKeyUpdate struct {
	Scope   *string `json:"scope,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
	Notes   *string `json:"notes,omitempty"`
}

type ApiKeyCreatedResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Key   string `json:"key" example:"shown ONLY once — store it securely"`
}

// ---- Bulk users ----

type BulkUserResult struct {
	Index    int    `json:"index"`
	Username string `json:"username"`
	Status   string `json:"status"` // created | error
	Error    string `json:"error,omitempty"`
}

type BulkUserResponse struct {
	Total   int              `json:"total"`
	Created int              `json:"created"`
	Failed  int              `json:"failed"`
	Results []BulkUserResult `json:"results"`
}

// ---- Dashboard stats ----

type StatsOverview struct {
	Users          int64 `json:"users"`
	Groups         int64 `json:"groups"`
	NAS            int64 `json:"nas"`
	ActiveSessions int64 `json:"active_sessions"`
	SessionsToday  int64 `json:"sessions_today"`
	InputBytes24h  int64 `json:"input_bytes_24h"`
	OutputBytes24h int64 `json:"output_bytes_24h"`
	AuthAccept24h  int64 `json:"auth_accept_24h"`
	AuthReject24h  int64 `json:"auth_reject_24h"`
}

type TopUser struct {
	Username    string `json:"username"`
	InputBytes  int64  `json:"input_bytes"`
	OutputBytes int64  `json:"output_bytes"`
	TotalBytes  int64  `json:"total_bytes"`
}

type AuthSummary struct {
	SinceHours int64 `json:"since_hours"`
	Accept     int64 `json:"accept"`
	Reject     int64 `json:"reject"`
}

// ---- IP pool ----

type PoolSummary struct {
	PoolName  string `json:"pool_name"`
	Total     int64  `json:"total"`
	Allocated int64  `json:"allocated"`
	Available int64  `json:"available"`
}

// ---- Webhooks ----

type WebhookCreate struct {
	Name   string `json:"name" example:"billing-listener"`
	URL    string `json:"url" example:"https://billing.example.com/hooks/radius"`
	Event  string `json:"event" example:"session_stop"`
	Secret string `json:"secret,omitempty" example:"shared-secret-for-hmac"`
}

type WebhookUpdate struct {
	URL     *string `json:"url,omitempty"`
	Event   *string `json:"event,omitempty"`
	Secret  *string `json:"secret,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type WebhookCreatedResponse struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Event  string `json:"event"`
	URL    string `json:"url"`
	Secret string `json:"secret" example:"shown ONLY once — store it securely"`
}

// ====== Legacy /api/v1/* compatibility schemas (nasvpntest-api) ======
//
// Mengikuti shape FastAPI lama: row mentah dengan field id di akhir.

type V1RowIn struct {
	Username  string `json:"username,omitempty"`
	Groupname string `json:"groupname,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Op        string `json:"op,omitempty"`
	Value     string `json:"value,omitempty"`
	Priority  *int   `json:"priority,omitempty"`
}

type V1Radcheck struct {
	Username  string `json:"username"`
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	ID        uint   `json:"id"`
}

type V1Radreply struct {
	Username  string `json:"username"`
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	ID        uint   `json:"id"`
}

type V1Radgroupcheck struct {
	Groupname string `json:"groupname"`
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	ID        uint   `json:"id"`
}

type V1Radgroupreply struct {
	Groupname string `json:"groupname"`
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	ID        uint   `json:"id"`
}

type V1Radusergroup struct {
	Username  string `json:"username"`
	Groupname string `json:"groupname"`
	Priority  int    `json:"priority"`
	ID        uint   `json:"id"`
}

type V1NAS struct {
	NASName     string `json:"nasname"`
	ShortName   string `json:"shortname"`
	Type        string `json:"type"`
	Ports       int    `json:"ports"`
	Secret      string `json:"secret"`
	Server      string `json:"server"`
	Community   string `json:"community"`
	Description string `json:"description"`
	ID          uint   `json:"id"`
}

type V1NASCreate struct {
	NASName     string `json:"nasname"`
	ShortName   string `json:"shortname,omitempty"`
	Type        string `json:"type,omitempty"`
	Ports       int    `json:"ports,omitempty"`
	Secret      string `json:"secret"`
	Server      string `json:"server,omitempty"`
	Community   string `json:"community,omitempty"`
	Description string `json:"description,omitempty"`
}

type V1Radacct struct {
	AcctSessionID      string  `json:"acctsessionid"`
	AcctUniqueID       string  `json:"acctuniqueid"`
	Username           string  `json:"username"`
	Realm              string  `json:"realm"`
	NASIPAddress       string  `json:"nasipaddress"`
	NASPortID          string  `json:"nasportid"`
	NASPortType        string  `json:"nasporttype"`
	AcctStartTime      *string `json:"acctstarttime"`
	AcctUpdateTime     *string `json:"acctupdatetime"`
	AcctStopTime       *string `json:"acctstoptime"`
	AcctInterval       int     `json:"acctinterval"`
	AcctSessionTime    int     `json:"acctsessiontime"`
	AcctAuthentic      string  `json:"acctauthentic"`
	ConnectInfoStart   string  `json:"connectinfo_start"`
	ConnectInfoStop    string  `json:"connectinfo_stop"`
	AcctInputOctets    int64   `json:"acctinputoctets"`
	AcctOutputOctets   int64   `json:"acctoutputoctets"`
	CalledStationID    string  `json:"calledstationid"`
	CallingStationID   string  `json:"callingstationid"`
	AcctTerminateCause string  `json:"acctterminatecause"`
	ServiceType        string  `json:"servicetype"`
	FramedProtocol     string  `json:"framedprotocol"`
	FramedIPAddress    string  `json:"framedipaddress"`
	RadAcctID          uint64  `json:"radacctid"`
}

type V1UserStatus struct {
	Username string `json:"username"`
	Online   bool   `json:"online"`
	// IsOnline — nama yang DIBACA pemanggil (backend ERP men-decode ke
	// `json:"is_online"`). Tanpa field ini, json.Unmarshal mengabaikan `online`
	// dan IsOnline SELALU false — tanpa galat, tanpa jejak. Akibatnya cabang
	// "pelanggan ONLINE tapi sesinya terdaftar dengan username lain" tak pernah
	// tercapai, sehingga isolir yang tidak menggigit tercatat sebagai kabar
	// jinak "pelanggan tidak online". Dikirim BERDUA (bukan diganti) supaya
	// pemanggil lama yang membaca `online` tetap jalan.
	IsOnline bool       `json:"is_online"`
	Session  *V1Radacct `json:"session"`
}

type V1DisconnectRequest struct {
	Username     string `json:"username"`
	NASIPAddress string `json:"nas_ip_address,omitempty"`
	Port         int    `json:"port,omitempty"`
	TimeoutMs    int    `json:"timeout_ms,omitempty"`
}

type V1DisconnectResponse struct {
	Status        string `json:"status"`
	Code          string `json:"code"`
	Username      string `json:"username"`
	NASIPAddress  string `json:"nasipaddress"`
	Port          int    `json:"port"`
	AcctSessionID string `json:"acctsessionid"`

	// ── Bentuk lama (aplikasi Python) — dikirim BERSAMAAN, bukan pengganti ──
	//
	// Pemanggil yang sudah ada membaca nama-nama ini. Yang paling menentukan
	// adalah `output`: alat diagnosa cmd/periksa-jalur-kick memutuskan sebuah
	// NAS "HIDUP (router menjawab)" HANYA dari strings.Contains(Output,
	// "Received Disconnect"). Kalau kosong, SETIAP NAS dilaporkan tak menjawab —
	// negatif palsu yang menyuruh operator mengejar kerusakan yang tak ada.
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	ReturnCode int    `json:"return_code"`
	NasIP      string `json:"nas_ip"`
}
