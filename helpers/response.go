package helpers

import "github.com/gin-gonic/gin"

// Success mengirim respons sukses dengan format seragam
func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// Error mengirim respons error dengan format seragam
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
	})
}
