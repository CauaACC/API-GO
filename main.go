package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Sala struct {
	ID         string   `json:"id"`
	Nome       string   `json:"nome"`
	Capacidade int      `json:"capacidade"`
	Recursos   []string `json:"recursos"`
}

type Aluno struct {
	Matricula string `json:"matricula"`
	Nome      string `json:"nome"`
	Email     string `json:"email"`
}

type Alocacao struct {
	SalaID     string `json:"sala_id"`
	DiaSemana  string `json:"dia_semana"`
	HoraInicio string `json:"hora_inicio"`
	HoraFim    string `json:"hora_fim"`
}

type Turma struct {
	ID         string    `json:"id"`
	Nome       string    `json:"nome"`
	Disciplina string    `json:"disciplina"`
	Docente    string    `json:"docente"`
	Alunos     []string  `json:"alunos"`
	Alocacao   *Alocacao `json:"alocacao,omitempty"`
}

type turmaView struct {
	ID         string    `json:"id"`
	Nome       string    `json:"nome"`
	Disciplina string    `json:"disciplina"`
	Docente    string    `json:"docente"`
	QtdAlunos  int       `json:"qtd_alunos"`
	Alocada    bool      `json:"alocada"`
	Alocacao   *Alocacao `json:"alocacao,omitempty"`
}

var (
	mu     sync.Mutex
	salas  = map[string]*Sala{}
	alunos = map[string]*Aluno{}
	turmas = map[string]*Turma{}
)

const version = "1.0.0"

func normalizarDia(d string) string {
	return strings.ToLower(strings.TrimSpace(d))
}

func horarioValido(h string) bool {
	if len(h) != 5 || h[2] != ':' {
		return false
	}
	hh := h[:2]
	mm := h[3:]
	for _, c := range hh + mm {
		if c < '0' || c > '9' {
			return false
		}
	}
	if hh > "23" || mm > "59" {
		return false
	}
	return true
}

func sobrepoe(iniA, fimA, iniB, fimB string) bool {
	return iniA < fimB && fimA > iniB
}

func main() {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", getHealth)

		v1.POST("/salas", criarSala)
		v1.GET("/salas", listarSalas)

		v1.POST("/alunos", criarAluno)
		v1.GET("/alunos", listarAlunos)
		v1.GET("/alunos/:matricula", buscarAluno)

		v1.GET("/salas/:id/grade", gradeSala)

		v1.POST("/turmas", criarTurma)
		v1.GET("/turmas", listarTurmas)
		v1.POST("/turmas/:id/alunos", matricularAluno)
		v1.GET("/turmas/:id/alunos", listarAlunosTurma)
		v1.POST("/turmas/:id/alocar", alocarSala)
	}

	r.Run(":8080")
}

func getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   version,
	})
}

func criarSala(c *gin.Context) {
	var in Sala
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "payload inválido"})
		return
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Nome) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "id e nome são obrigatórios"})
		return
	}
	if in.Capacidade <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "capacidade deve ser maior que zero"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := salas[in.ID]; ok {
		c.JSON(http.StatusConflict, gin.H{"erro": "sala já cadastrada"})
		return
	}
	if in.Recursos == nil {
		in.Recursos = []string{}
	}
	salas[in.ID] = &in
	c.JSON(http.StatusCreated, in)
}

func listarSalas(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Sala, 0, len(salas))
	for _, s := range salas {
		out = append(out, *s)
	}
	c.JSON(http.StatusOK, out)
}

func criarAluno(c *gin.Context) {
	var in Aluno
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "payload inválido"})
		return
	}
	if strings.TrimSpace(in.Matricula) == "" || strings.TrimSpace(in.Nome) == "" || strings.TrimSpace(in.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "matricula, nome e email são obrigatórios"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := alunos[in.Matricula]; ok {
		c.JSON(http.StatusConflict, gin.H{"erro": "aluno já cadastrado"})
		return
	}
	alunos[in.Matricula] = &in
	c.JSON(http.StatusCreated, in)
}

func listarAlunos(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Aluno, 0, len(alunos))
	for _, a := range alunos {
		out = append(out, *a)
	}
	c.JSON(http.StatusOK, out)
}

func buscarAluno(c *gin.Context) {
	matricula := c.Param("matricula")

	mu.Lock()
	defer mu.Unlock()

	aluno, ok := alunos[matricula]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "aluno não encontrado"})
		return
	}
	c.JSON(http.StatusOK, aluno)
}

func gradeSala(c *gin.Context) {
	salaID := c.Param("id")
	filtroDia := normalizarDia(c.Query("dia"))

	mu.Lock()
	defer mu.Unlock()

	if _, ok := salas[salaID]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "sala não encontrada"})
		return
	}

	type slot struct {
		TurmaID    string `json:"turma_id"`
		TurmaNome  string `json:"turma_nome"`
		Disciplina string `json:"disciplina"`
		DiaSemana  string `json:"dia_semana"`
		HoraInicio string `json:"hora_inicio"`
		HoraFim    string `json:"hora_fim"`
	}

	out := make([]slot, 0)
	for _, t := range turmas {
		if t.Alocacao == nil || t.Alocacao.SalaID != salaID {
			continue
		}
		if filtroDia != "" && normalizarDia(t.Alocacao.DiaSemana) != filtroDia {
			continue
		}
		out = append(out, slot{
			TurmaID:    t.ID,
			TurmaNome:  t.Nome,
			Disciplina: t.Disciplina,
			DiaSemana:  t.Alocacao.DiaSemana,
			HoraInicio: t.Alocacao.HoraInicio,
			HoraFim:    t.Alocacao.HoraFim,
		})
	}
	c.JSON(http.StatusOK, out)
}

func criarTurma(c *gin.Context) {
	var in Turma
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "payload inválido"})
		return
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Nome) == "" ||
		strings.TrimSpace(in.Disciplina) == "" || strings.TrimSpace(in.Docente) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "id, nome, disciplina e docente são obrigatórios"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := turmas[in.ID]; ok {
		c.JSON(http.StatusConflict, gin.H{"erro": "turma já cadastrada"})
		return
	}
	in.Alunos = []string{}
	in.Alocacao = nil
	turmas[in.ID] = &in
	c.JSON(http.StatusCreated, in)
}

func listarTurmas(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	out := make([]turmaView, 0, len(turmas))
	for _, t := range turmas {
		out = append(out, turmaView{
			ID:         t.ID,
			Nome:       t.Nome,
			Disciplina: t.Disciplina,
			Docente:    t.Docente,
			QtdAlunos:  len(t.Alunos),
			Alocada:    t.Alocacao != nil,
			Alocacao:   t.Alocacao,
		})
	}
	c.JSON(http.StatusOK, out)
}

func matricularAluno(c *gin.Context) {
	turmaID := c.Param("id")
	var body struct {
		AlunoID string `json:"aluno_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.AlunoID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "aluno_id é obrigatório"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	turma, ok := turmas[turmaID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "turma não encontrada"})
		return
	}
	if _, ok := alunos[body.AlunoID]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "aluno não encontrado"})
		return
	}
	for _, m := range turma.Alunos {
		if m == body.AlunoID {
			c.JSON(http.StatusConflict, gin.H{"erro": "aluno já matriculado nesta turma"})
			return
		}
	}
	if turma.Alocacao != nil {
		sala := salas[turma.Alocacao.SalaID]
		if sala != nil && len(turma.Alunos)+1 > sala.Capacidade {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"erro": "capacidade da sala insuficiente"})
			return
		}
	}
	if turma.Alocacao != nil {
		for _, outra := range turmas {
			if outra.ID == turma.ID || outra.Alocacao == nil {
				continue
			}
			if normalizarDia(outra.Alocacao.DiaSemana) != normalizarDia(turma.Alocacao.DiaSemana) {
				continue
			}
			if !sobrepoe(turma.Alocacao.HoraInicio, turma.Alocacao.HoraFim,
				outra.Alocacao.HoraInicio, outra.Alocacao.HoraFim) {
				continue
			}
			for _, m := range outra.Alunos {
				if m == body.AlunoID {
					c.JSON(http.StatusConflict, gin.H{"erro": "conflito de agenda do aluno"})
					return
				}
			}
		}
	}

	turma.Alunos = append(turma.Alunos, body.AlunoID)
	c.JSON(http.StatusOK, turma)
}

func listarAlunosTurma(c *gin.Context) {
	turmaID := c.Param("id")

	mu.Lock()
	defer mu.Unlock()

	turma, ok := turmas[turmaID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "turma não encontrada"})
		return
	}
	out := make([]Aluno, 0, len(turma.Alunos))
	for _, mat := range turma.Alunos {
		if a, ok := alunos[mat]; ok {
			out = append(out, *a)
		}
	}
	c.JSON(http.StatusOK, out)
}

func alocarSala(c *gin.Context) {
	turmaID := c.Param("id")
	var in Alocacao
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "payload inválido"})
		return
	}
	if strings.TrimSpace(in.SalaID) == "" || strings.TrimSpace(in.DiaSemana) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "sala_id e dia_semana são obrigatórios"})
		return
	}
	if !horarioValido(in.HoraInicio) || !horarioValido(in.HoraFim) {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "horário deve estar no formato HH:MM"})
		return
	}
	if in.HoraInicio >= in.HoraFim {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "hora_inicio deve ser menor que hora_fim"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	turma, ok := turmas[turmaID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "turma não encontrada"})
		return
	}
	sala, ok := salas[in.SalaID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"erro": "sala não encontrada"})
		return
	}
	if len(turma.Alunos) > sala.Capacidade {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"erro": "capacidade da sala insuficiente para os alunos matriculados"})
		return
	}
	diaNovo := normalizarDia(in.DiaSemana)
	for _, outra := range turmas {
		if outra.ID == turma.ID || outra.Alocacao == nil {
			continue
		}
		if outra.Alocacao.SalaID != in.SalaID {
			continue
		}
		if normalizarDia(outra.Alocacao.DiaSemana) != diaNovo {
			continue
		}
		if sobrepoe(in.HoraInicio, in.HoraFim, outra.Alocacao.HoraInicio, outra.Alocacao.HoraFim) {
			c.JSON(http.StatusConflict, gin.H{"erro": "conflito de agenda: sala já alocada neste horário"})
			return
		}
	}
	for _, mat := range turma.Alunos {
		for _, outra := range turmas {
			if outra.ID == turma.ID || outra.Alocacao == nil {
				continue
			}
			if normalizarDia(outra.Alocacao.DiaSemana) != diaNovo {
				continue
			}
			if !sobrepoe(in.HoraInicio, in.HoraFim, outra.Alocacao.HoraInicio, outra.Alocacao.HoraFim) {
				continue
			}
			for _, m := range outra.Alunos {
				if m == mat {
					c.JSON(http.StatusConflict, gin.H{"erro": "conflito de agenda de aluno já matriculado"})
					return
				}
			}
		}
	}

	turma.Alocacao = &in
	c.JSON(http.StatusOK, turma)
}
