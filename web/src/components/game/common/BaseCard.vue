<template>
    <el-card :shadow="shadow" body-class="p-2! flex flex-col gap-2">
        <slot></slot>
    </el-card>
</template>

<script setup lang="ts">

const props = withDefaults(defineProps<{
    shadow?: "hover" | "always" | "never"
}>(), {
    shadow: "hover"
})

</script>