# Edge Design Rules

1. Sem alocação dinâmica no core
2. Estados são globais e únicos
3. Slots não se comunicam diretamente
4. Segurança sempre tem precedência
5. Gateway e Backend são externos ao Edge
6. Toda falha deve resultar em estado SAFE