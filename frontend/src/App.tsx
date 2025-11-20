import './App.css'
import Dashboard from './components/Dashboard'
import ConnectionStatusIndicator from './components/ConnectionStatus'

function App() {
  return (
    <div className="app">
      <header className="app-header">
        <h1>Smart City Monitor</h1>
        <p>Real-time sensor monitoring dashboard</p>
        <ConnectionStatusIndicator />
      </header>

      <main className="app-main">
        <Dashboard />
      </main>
    </div>
  )
}

export default App
