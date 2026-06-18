package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GenerateID() string {
	return uuid.New().String()
}

func SHA256Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func NormalRandom(mean, stdDev float64) float64 {
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return mean + z*stdDev
}

func IPToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func IPInRange(clientIP, startIP, endIP string) bool {
	client := net.ParseIP(clientIP)
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)
	if client == nil || start == nil || end == nil {
		return false
	}
	clientInt := IPToInt(client)
	startInt := IPToInt(start)
	endInt := IPToInt(end)
	return clientInt >= startInt && clientInt <= endInt
}

func IPInCIDR(clientIP, cidr string) bool {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	return ipnet.Contains(ip)
}

func InTimeWindow(startTime, endTime string) bool {
	now := time.Now()
	currentTime := now.Format("15:04:05")

	if startTime <= endTime {
		return currentTime >= startTime && currentTime <= endTime
	}
	return currentTime >= startTime || currentTime <= endTime
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func GetClientIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

func StringSliceContains(slice []string, s string, ignoreCase bool) bool {
	for _, item := range slice {
		if ignoreCase {
			if strings.EqualFold(item, s) {
				return true
			}
		} else {
			if item == s {
				return true
			}
		}
	}
	return false
}

func PathToSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func GetPathSegment(path string, index int) string {
	segments := PathToSegments(path)
	if index < 0 || index >= len(segments) {
		return ""
	}
	return segments[index]
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
