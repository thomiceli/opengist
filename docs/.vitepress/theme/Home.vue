<script>
import { withBase } from 'vitepress';
import './theme.css'

export default {

  setup() {
    return { withBase };
  },
};

</script>


<template>
  <main class="home">
    <header class="hero">
      <div class="mx-auto max-w-7xl px-6 lg:px-8">
        <div class="mx-auto  lg:text-center">
          <img class="rotating h-36 mx-auto my-8 " src="https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg" alt="" >
          <a target="_blank" href="https://github.com/thomiceli/opengist/releases" class="inline-flex items-center rounded-full bg-indigo-100 hover:bg-indigo-200 px-4 py-1.5 text-lg font-medium text-indigo-700">
              <span class="pr-1">Released 1.10</span>
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
              <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 19.5 15-15m0 0H8.25m11.25 0v11.25" />
            </svg>
            </a>
            <h1 class="mt-5 text-4xl font-bold tracking-tight sm:text-5xl">Opengist</h1>
            <h2 class="mt-4 text-xl">Self-hosted pastebin powered by Git, open-source alternative to Github Gist.</h2>
          </div>
        <div class="space-x-2 my-12">
          <a href="/docs" class="rounded-md bg-indigo-600 mt-6 px-5 py-3 text-xl font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">Docs</a>
          <a target="_blank" href="https://demo.opengist.io" class="rounded-md bg-indigo-400 mt-6 px-5 py-3 text-xl border-white font-semibold text-white shadow-sm hover:bg-indigo-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">Live demo</a>
          <a target="_blank" href="https://github.com/thomiceli/opengist" class="rounded-md bg-gray-800 mt-6 px-3 py-3 text-xl dark:border dark:border-1 dark:border-gray-400 font-semibold text-white shadow-sm hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">
            <svg stroke="currentColor" fill="currentColor" stroke-width="0" viewBox="0 0 16 16" class="w-7 h-auto inline" height="1em" width="1em" xmlns="http://www.w3.org/2000/svg"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8"></path></svg>
          </a>
        </div>
        <div class="border border-1 mt-6 px-5 py-3 rounded-md shadow-sm ">
          <code class="select-all ">docker run --name <span class="text-indigo-700 dark:text-indigo-300 font-bold">opengist</span> -p <span class="text-indigo-700 dark:text-indigo-300 font-bold">6157</span>:6157 -v "<span class="text-indigo-700 dark:text-indigo-300 font-bold">$HOME/.opengist</span>:/opengist" ghcr.io/thomiceli/opengist:1</code>
        </div>
      </div>
    </header>

    <div class="relative w-full sm:max-w-7xl mx-auto overflow-auto">
      <img class="block w-[200vw] max-w-none sm:w-full h-auto" :src="withBase('/opengist-demo.png')" alt="demo-opengist-screenshot" />
    </div>

  </main>
</template>



<style>
@-webkit-keyframes rotating /* Safari and Chrome */ {
  from {
    -webkit-transform: rotate(0deg);
    -o-transform: rotate(0deg);
    transform: rotate(0deg);
  }
  to {
    -webkit-transform: rotate(360deg);
    -o-transform: rotate(360deg);
    transform: rotate(360deg);
  }
}
@keyframes rotating {
  from {
    -ms-transform: rotate(0deg);
    -moz-transform: rotate(0deg);
    -webkit-transform: rotate(0deg);
    -o-transform: rotate(0deg);
    transform: rotate(0deg);
  }
  to {
    -ms-transform: rotate(360deg);
    -moz-transform: rotate(360deg);
    -webkit-transform: rotate(360deg);
    -o-transform: rotate(360deg);
    transform: rotate(360deg);
  }
}

.home {

  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  text-align: center;
}

.rotating {
  -webkit-animation: rotating 8s linear infinite;
  -moz-animation: rotating 4s linear infinite;
  -ms-animation: rotating 4s linear infinite;
  -o-animation: rotating 4s linear infinite;
  animation: rotating 12s linear infinite;
}


</style>