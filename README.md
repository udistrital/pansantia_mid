# pasantia_mid (antes: castor_mid)

Servicio **MID (Middle Layer)** de la **Plataforma de Pasantías** de la Universidad Distrital Francisco José de Caldas.  
Este microservicio centraliza reglas de negocio y orquestación entre:

- **pasantia_crud** (persistencia / modelos / BD)
- Servicios OAS/UD (Autenticación, Terceros, Oikos/Dependencias, Documentos, Notificaciones, etc.)

> **Nota:** Este repositorio fue renombrado desde `castor_mid` a `pasantia_mid`.  
> Si tu aplicación cliente aún referencia “castor”, revisa variables de entorno y rutas base.

---


---

## Arquitectura

**pasantia_mid** implementa la capa de negocio/orquestación:

- Normaliza y valida payloads.
- Enriquecimiento de datos (join lógico entre fuentes).
- Manejo de estados (ofertas, postulaciones, invitaciones).
- Homologación de proyecto curricular (código académica → id_oikos + nombre).
- Enrutamiento de flujos por rol (Estudiante / Tutor Externo).

---

## Requisitos

- **Go** (compatible con el `go.mod` del proyecto)
- **Bee / Beego CLI** (si usas `bee run`)
- Acceso a:
  - **pasantia_crud** (URL base)
  - APIs OAS/UD requeridas

Instalar Bee (si aplica):
```bash
go install github.com/beego/bee/v2@latest
