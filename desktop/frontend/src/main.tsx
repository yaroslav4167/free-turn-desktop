import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import LocalizedApp from './LocalizedApp'
import ErrorBoundary from './ErrorBoundary'
import {initializeTheme} from './theme'

initializeTheme()

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ErrorBoundary>
            <LocalizedApp/>
        </ErrorBoundary>
    </React.StrictMode>
)
