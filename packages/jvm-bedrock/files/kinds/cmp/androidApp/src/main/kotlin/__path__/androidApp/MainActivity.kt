package __package__.androidApp

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent

class MainActivity : ComponentActivity() {
  override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    // The full name keeps the imports sorted whatever the group is.
    setContent { __package__.shared.Greeting("Android") }
  }
}
