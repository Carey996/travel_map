package handler

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 测试用的固定用户：validStorageKey 只关心字符串前缀，不需要真实数据库。
var (
	testUser  = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	otherUser = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

func TestValidStorageKey(t *testing.T) {
	own := "u/" + testUser.String() + "/"
	theirs := "u/" + otherUser.String() + "/"

	cases := []struct {
		name string
		user uuid.UUID
		key  string
		want bool
	}{
		// ---- 正例：服务端签发的对象键 ----
		{"原图键", testUser, own + "a1b2c3d4e5f6a7b8.jpg", true},
		{"预览键", testUser, own + "a1b2c3d4e5f6a7b8.png.preview.jpg", true},
		{"缩略图键", testUser, own + "a1b2c3d4e5f6a7b8.png.thumb.jpg", true},

		// ---- 反例：不属于当前用户 / 非法键 ----
		{"空键", testUser, "", false},
		{"只有前缀", testUser, own, false},
		{"他人前缀", testUser, theirs + "a1b2c3d4e5f6a7b8.jpg", false},
		{"缺少 u/ 前缀", testUser, "a1b2c3d4e5f6a7b8.jpg", false},
		{"绝对路径", testUser, "/" + own + "a1b2c3d4e5f6a7b8.jpg", false},
		{"路径穿越到他人目录", testUser, own + "../" + otherUser.String() + "/a.jpg", false},
		{"前缀后多余斜杠", testUser, own + "/a.jpg", false},
		{"反斜杠", testUser, own + `..\x.jpg`, false},
		{"NUL 字节", testUser, own + "a\x00.jpg", false},
		{"前缀大小写不符", testUser, "U/" + testUser.String() + "/a.jpg", false},
		{"nil 用户", uuid.Nil, "u/" + uuid.Nil.String() + "/a.jpg", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validStorageKey(tc.user, tc.key); got != tc.want {
				t.Fatalf("validStorageKey(%q, %q) = %v, want %v", tc.user, tc.key, got, tc.want)
			}
		})
	}
}

func TestValidContentHash(t *testing.T) {
	sha256 := strings.Repeat("ab12", 16) // 64 位十六进制

	cases := []struct {
		name string
		hash string
		want bool
	}{
		{"64 位小写十六进制", sha256, true},
		{"64 位大写十六进制", strings.ToUpper(sha256), true},
		{"8 位十六进制", "a1b2c3d4", true},
		{"7 位太短", "a1b2c3d", false},
		{"65 位太长", sha256 + "a", false},
		{"空哈希", "", false},
		{"含路径分隔符", "../../a1b2c3d4", false},
		{"含非十六进制字符", "a1b2c3d4zz", false},
		{"含空格", "a1b2 c3d4", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validContentHash(tc.hash); got != tc.want {
				t.Fatalf("validContentHash(%q) = %v, want %v", tc.hash, got, tc.want)
			}
		})
	}
}
