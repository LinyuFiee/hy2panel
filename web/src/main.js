import { createApp, h } from 'vue'
import App from './App.vue'
import { Toaster } from '@/components/ui/sonner'
import './styles/globals.css'
import 'vue-sonner/style.css'
import './styles/theme'

const Root = {
  render() {
    return [h(App), h(Toaster)]
  }
}

createApp(Root).mount('#app')
