package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {

	r := gin.New()

	// Uso dos Middlewares globais nativos e personalizados
	r.Use(gin.Recovery())

	// 4. Mapeamento de Rotas sob Grupo Versionado
	v1 := r.Group("/api/v1")
	{
		// Monitoramento da API
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now(),
				"version":   "1.0.0",
			})
		})

		// Domínio de Turmas (Classes)
		//v1.POST("/turmas", turmaHandler.CriarTurma)
		//v1.GET("/turmas", turmaHandler.ListarTurmas)
		//v1.POST("/turmas/:id/alocar", turmaHandler.AlocarSala)
	}

	r.Run(":8080")
}
