---
aside: false
pageClass: api-page
---
<script setup>
import { useData } from 'vitepress'
const { params } = useData()
</script>

<OpenApiOperation :id="params.id" />
