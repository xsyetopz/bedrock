package __package__.shared

import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable

/** Returns a greeting for [name]. */
public fun greet(name: String): String = "Hello, $name"

/** Shows a greeting for [name]. */
@Composable
public fun Greeting(name: String) {
  BasicText(greet(name))
}
