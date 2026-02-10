package routers

import (
	"github.com/udistrital/pasantia_mid/controllers/errorhandler"
	internalcontrollers "github.com/udistrital/pasantia_mid/internal/controllers"

	beego "github.com/beego/beego/v2/server/web"
)

func init() {
	// Manejador de errores
	beego.ErrorController(&errorhandler.ErrorHandlerController{})

	beego.Router("/v1/ofertas", &internalcontrollers.OfertaController{}, "post:PostCrear")
	beego.Router("/v1/ofertas/abiertas", &internalcontrollers.OfertaController{}, "get:GetAbiertas")
	beego.Router("/v1/ofertas/en-curso", &internalcontrollers.OfertaController{}, "get:GetEnCurso")
	beego.Router("/v1/ofertas/:id/cancelar", &internalcontrollers.OfertaController{}, "put:PutCancelar")
	beego.Router("/v1/ofertas/:id/finalizar", &internalcontrollers.OfertaController{}, "put:PutFinalizar")
	beego.Router("/v1/ofertas/:id/pausar", &internalcontrollers.OfertaController{}, "put:Pausar")
	beego.Router("/v1/ofertas/:id/reactivar", &internalcontrollers.OfertaController{}, "put:PutReactivar")
	beego.Router("/v1/ofertas", &internalcontrollers.OfertaController{}, "get:GetListado")
	beego.Router("/v1/ofertas/:id/postulaciones", &internalcontrollers.PostulacionesController{}, "get:GetByOferta")
	beego.Router("/v1/ofertas/:id/postular", &internalcontrollers.PostulacionesEstudianteController{}, "post:PostPostularOferta")
	beego.Router("/v1/ofertas/:id", &internalcontrollers.OfertaController{}, "get:GetById")

	beego.Router("/v1/ofertas/:id/proyectos_curriculares", &internalcontrollers.OfertaPCController{}, "get:GetList;post:PostBulk")
	beego.Router("/v1/ofertas/:id/proyectos_curriculares/:pcId", &internalcontrollers.OfertaPCController{}, "delete:DeleteOne")

	beego.Router("/v1/postulaciones/:id/accion", &internalcontrollers.PostulacionesController{}, "post:PostAccion")
	beego.Router("/v1/postulaciones/:id/visto", &internalcontrollers.PostulacionesController{}, "put:PutVisto")

	beego.Router("/v1/estudiantes/perfil", &internalcontrollers.EstudiantesController{}, "get:GetMiPerfil;post:PostUpsertPerfil;put:PutActualizarPerfil")
	beego.Router("/v1/estudiantes/perfil/visibilidad", &internalcontrollers.EstudiantesController{}, "put:PutVisibilidad")
	beego.Router("/v1/estudiantes/perfil/cv", &internalcontrollers.EstudiantesController{}, "put:PutCV")
	beego.Router("/v1/estudiantes/perfil/visitas", &internalcontrollers.EstudiantesController{}, "get:GetVisitas")
	beego.Router("/v1/estudiantes/perfil/consulta_documento", &internalcontrollers.EstudiantesController{}, "post:PostPerfilPorDocumento")
	beego.Router("/v1/estudiantes/invitaciones", &internalcontrollers.InvitacionesController{}, "get:GetBandejaEstudiante")
	beego.Router("/v1/estudiantes/postulaciones", &internalcontrollers.PostulacionesEstudianteController{}, "get:GetMisPostulaciones")
	beego.Router("/v1/estudiantes/postulaciones/:id/aceptar-seleccion", &internalcontrollers.PostulacionesController{}, "put:PutAceptarSeleccion")
	beego.Router("/v1/estudiantes/postulaciones/:id", &internalcontrollers.PostulacionesEstudianteController{}, "get:GetById")
	beego.Router("/v1/estudiantes/dashboard", &internalcontrollers.DashboardController{}, "get:GetEstudiante")

	beego.Router("/v1/explorar/estudiantes", &internalcontrollers.ExplorarController{}, "get:GetCatalogo")
	beego.Router("/v1/explorar/estudiantes/:perfil_id", &internalcontrollers.ExplorarController{}, "get:GetPerfil")
	beego.Router("/v1/explorar/estudiantes/:perfil_id/guardar", &internalcontrollers.ExplorarController{}, "post:PostGuardar;delete:DeleteGuardar")
	beego.Router("/v1/explorar/estudiantes/:perfil_id/visita", &internalcontrollers.ExplorarController{}, "post:PostVisita")
	beego.Router("/v1/explorar/estudiantes/:perfil_id/invitar", &internalcontrollers.InvitacionesController{}, "post:PostInvitar")

	beego.Router("/v1/tutores/invitaciones", &internalcontrollers.InvitacionesController{}, "get:GetBandejaTutor")
	beego.Router("/v1/tutores/dashboard", &internalcontrollers.DashboardController{}, "get:GetTutor")
	beego.Router("/v1/invitaciones/:id/aceptar", &internalcontrollers.InvitacionesController{}, "put:PutAceptar")
	beego.Router("/v1/invitaciones/:id/rechazar", &internalcontrollers.InvitacionesController{}, "put:PutRechazar")
	beego.Router("/v1/invitaciones/:id", &internalcontrollers.InvitacionesController{}, "get:GetById")

	beego.Router("/v1/catalogos/facultades", &internalcontrollers.CatalogosController{}, "get:GetFacultades")
	beego.Router("/v1/catalogos/facultades/:id/proyectos-curriculares", &internalcontrollers.CatalogosController{}, "get:GetPCPorFacultad")
	beego.Router("/v1/catalogos/proyectos-curriculares", &internalcontrollers.CatalogosController{}, "get:GetProyectosCurriculares")
	beego.Router("/v1/catalogos/proyectos-curriculares/:id", &internalcontrollers.CatalogosController{}, "get:GetProyectoCurricular")
	beego.Router("/v1/catalogos/proyecto-curricular", &internalcontrollers.CatalogosController{}, "get:GetProyectoCurricularPorCodigo")

	beego.Router("/v1/terceros/tutor_externo/registrar", &internalcontrollers.TercerosController{}, "post:PostRegistrarTutorExterno")
	beego.Router("/v1/terceros/empresa/:id", &internalcontrollers.TercerosController{}, "get:GetEmpresaByID")
	beego.Router("/v1/terceros/tutor/:id", &internalcontrollers.TercerosController{}, "get:GetTutorByID")
	beego.Router("/v1/tutores/estado", &internalcontrollers.TutoresController{}, "post:PostEstado")
	beego.Router("/v1/tutores/empresa", &internalcontrollers.TutoresController{}, "post:PostUpsertEmpresa")
}
