# Pending tasks

- Validate different birthday date formats in PUT auth/profile


Funcionalidades Faltantes (Opcionales)
1. Delegación de Ownership al Crear Tenant
Permitir que un super_admin cree un tenant y asigne a otro usuario como owner (en lugar de ser el creador automáticamente).

Caso de uso:

Beneficios:

Super_admin puede crear tenants para clientes sin ser owner
Útil para onboarding de nuevos clientes
Separación de roles: super_admin gestiona, clientes son owners
2. Otras Mejoras Opcionales (No en la lista)
a) Transferencia de Ownership:

Permitir que owner actual transfiera ownership a otro usuario
Útil cuando alguien deja la empresa
b) Update de Roles:

Cambiar el rol de un usuario existente
Actualmente solo puedes agregar/remover
c) Listar Usuarios del Tenant:

Ver todos los miembros del tenant con sus roles
Filtrar por rol activo/inactivo