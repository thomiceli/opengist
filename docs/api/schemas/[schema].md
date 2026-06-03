---
aside: false
---
<script setup>
import { useData } from 'vitepress'
const { params } = useData()
</script>

<OpenApiSchema :name="params.name" />
