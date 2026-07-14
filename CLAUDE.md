# Leloir CLI - Directivas

Este repositorio contiene la CLI (Command Line Interface) de Leloir.

## Reglas Arquitectónicas
- **Producto Público (OSS):** Este repositorio es 100% **PÚBLICO** y de código abierto.
- **Cliente HTTP:** La CLI actúa como un cliente puro de la API pública de Leloir (`leloir-core`). Interactúa exclusivamente mediante peticiones HTTP.
- **Lenguaje:** Escrito en Go (o el lenguaje que el backend defina) para ser un binario estático multiplataforma.

## Restricción de Agentes
- **Exclusividad de Código:** ESTE REPOSITORIO ES DOMINIO EXCLUSIVO DE CLAUDE. Solo Claude puede escribir, modificar o alterar el código fuente de la CLI. Antigravity no debe tocar el código.
